import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep, group } from 'k6';

// ========== 配置部分 ==========
export const options = {
  scenarios: {
    // HTTP压测：登录、发视频、发黄金评论
    http_scenario: {
      executor: 'constant-vus',
      exec: 'httpScenario',
      vus: 100, // 并发用户数
      duration: '30s',
    },
    // WebSocket压测：聊天室弹幕
    ws_scenario: {
      executor: 'constant-vus',
      exec: 'wsScenario',
      vus: 50, // 并发连接数
      duration: '30s',
      startTime: '5s', // 延迟5秒启动，避免和HTTP抢资源
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<800'], // 95%的请求小于800ms
  },
};

// ========== 基础配置 ==========
const BASE_URL = 'http://localhost:8080/api/v1';
const WS_BASE_URL = 'ws://localhost:8080/api/v1/ws/videos';

// ========== 工具函数 ==========
function randomString(length) {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars[Math.floor(Math.random() * chars.length)];
  }
  return result;
}

// ========== 场景1：HTTP 压测 ==========
export function httpScenario() {
  let authToken = ''; // 将 token 提升到顶层作用域  
  let createdVideoID = 0;

  group('1️⃣ 用户注册 + 登录', () => {
    const username = `user_${__VU}_${randomString(4)}`;
    const password = 'password123';

    // 注册
    const registerRes = http.post(`${BASE_URL}/users/register`, JSON.stringify({
      username, password,
    }), { headers: { 'Content-Type': 'application/json' } });

    check(registerRes, {
      '注册成功 (201 or 200)': (r) => r.status === 201 || r.status === 200,
    });

    // 登录
    const loginRes = http.post(`${BASE_URL}/users/login`, JSON.stringify({
      username, password,
    }), { headers: { 'Content-Type': 'application/json' } });

    check(loginRes, {
      '登录成功 (200)': (r) => r.status === 200,
      '返回token': (r) => r.json('token') !== '',
    });

    if (loginRes.status === 200 && loginRes.body) {
      const loginBody = loginRes.json();
      // 使用可选链 ?. 确保即使 data 或 token 不存在也不会报错
      authToken = loginBody?.data?.token || ''; 
    }
    check(authToken, { '成功提取到非空Token': (t) => t.length > 0 });
    sleep(0.5);


    group('2️⃣ 发布视频 + 黄金评论', () => {
      const videoPayload = JSON.stringify({
        title: `My Test Video by ${username}`,
        description: '压测视频上传',
        video_url: 'https://placeholder.com/video.mp4',
        cover_url: 'https://placeholder.com/cover.jpg',
      });

      const videoRes = http.post(`${BASE_URL}/videos`, videoPayload, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json',
        },
      });

      check(videoRes, {
        '视频发布成功': (r) => r.status === 201,
      });
      // 先将整个 JSON 响应体完整地解析成一个 JavaScript 对象，然后再用标准的 JavaScript 语法去访问这个对象的嵌套属性，从而精确地提取出了我们需要的 id
      // 【关键修复】使用和Token一样的精确解析方法来获取videoId
    if (videoRes.status === 201 && videoRes.body) {
        const videoBody = videoRes.json();
        createdVideoID = videoBody?.data?.id || 0;
    }

      const goldenPayload = JSON.stringify({
        content: `黄金评论 by ${username}`,
      });

      const goldenRes = http.post(`${BASE_URL}/videos/${createdVideoID}/golden_comment`, goldenPayload, {
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'Content-Type': 'application/json',
        },
      });

      check(goldenRes, {
        '黄金评论发布成功 (201)': (r) => r.status === 201,
      });
    });
  });

  sleep(1);
}

// ========== 场景2：WebSocket 压测 ==========
export function wsScenario() {
  const videoId = Math.floor(Math.random() * 50) + 1; // 随机选一个视频
  const url = `${WS_BASE_URL}/${videoId}`;

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', function () {
      console.log(`VU ${__VU} connected to video ${videoId}`);
      socket.send(`Hello from user_${__VU}`);
    });

    socket.on('message', function (msg) {
      console.log(`VU ${__VU} received: ${msg}`);
    });

    socket.on('close', function () {
      console.log(`VU ${__VU} disconnected`);
    });

    // 模拟发送多条消息
    for (let i = 0; i < 3; i++) {
      socket.send(`弹幕 ${i} from user_${__VU}`);
      sleep(1);
    }

    // 在线5秒后断开
    socket.setTimeout(function () {
      socket.close();
    }, 5000);
  });

  check(res, { 'WebSocket 连接成功 (101)': (r) => r && r.status === 101 });
}

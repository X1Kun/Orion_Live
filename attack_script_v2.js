import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep, group } from 'k6';
import { SharedArray } from 'k6/data';
import { Counter, Trend } from 'k6/metrics';

// ===================================================================================
// ⚙️ 1. 配置部分 (Configuration)
// ===================================================================================
export const options = {
  scenarios: {
    // 场景A: HTTP 压测提升到 500 个并发用户，持续 2 分钟
    http_realistic_scenario: {
      executor: 'constant-vus',
      exec: 'httpScenario',
      vus: 500,        // <<< 数量级提升！
      duration: '2m',    // <<< 延长压测时间
    },
    // 场景B: WebSocket 压测提升到 200 个并发连接，持续 2 分钟
    ws_chatroom_scenario: {
      executor: 'constant-vus',
      exec: 'wsScenario',
      vus: 200,        // <<< 数量级提升！
      duration: '2m',    // <<< 延长压测时间
      startTime: '10s',  // 稍微延长启动延迟
    },
  },
  // 阈值也需要相应调整，因为负载更高，延迟可能会增加
  thresholds: {
    'http_req_duration': ['p(95)<1000'], // 目标放宽到 1000ms
    'checks': ['rate>0.95'],             // 允许少量逻辑失败
  },
};

// ===================================================================================
// 🛠️ 2. 基础配置与自定义指标 (Base Config & Custom Metrics)
// ===================================================================================
const BASE_URL = 'http://localhost:8080/api/v1';
const WS_BASE_URL = 'ws://localhost:8080/api/v1/ws/videos';
const USER_COUNT = options.scenarios.http_realistic_scenario.vus || 100; // 预创建的用户数等于并发数

// 自定义WebSocket指标
const wsMessagesReceived = new Counter('ws_messages_received');
const wsConnectionDuration = new Trend('ws_connection_duration');

// ===================================================================================
// 📦 3. 数据准备与清理 (Setup & Teardown) - 并行优化版
// ===================================================================================
export function setup() {
  console.log(`==== 🚀 K6 Setup: Pre-creating ${USER_COUNT} test users in parallel... ====`);
  const users = [];
  const requests = {}; // 用于存放批量请求

  // 1. 准备所有注册请求
  for (let i = 1; i <= USER_COUNT; i++) {
    const username = `testuser_${i}_${randomString(5)}`;
    const password = 'password123';
    const payload = JSON.stringify({ username, password });
    // 为每个请求生成一个唯一的key
    requests[`register_${username}`] = {
      method: 'POST',
      url: `${BASE_URL}/users/register`,
      body: payload,
      params: { headers: { 'Content-Type': 'application/json' } },
    };
  }
  
  // 2.【关键】一次性、并行地发送所有注册请求
  const registerResponses = http.batch(requests);

  // 3. 准备所有登录请求 (只为那些注册成功的用户)
  const loginRequests = {};
  for (const key in registerResponses) {
    const res = registerResponses[key];
    if (res.status === 201 || res.status === 200) {
      // 从请求的body中解析出用户名和密码
      const reqBody = JSON.parse(requests[key].body);
      loginRequests[`login_${reqBody.username}`] = {
        method: 'POST',
        url: `${BASE_URL}/users/login`,
        body: JSON.stringify({ username: reqBody.username, password: reqBody.password }),
        params: { headers: { 'Content-Type': 'application/json' } },
      };
    }
  }

  // 4.【关键】一次性、并行地发送所有登录请求
  const loginResponses = http.batch(loginRequests);

  // 5. 收集最终结果
  for (const key in loginResponses) {
    const res = loginResponses[key];
    if (res.status === 200 && res.body) {
      const token = res.json()?.data?.token || '';
      if (token) {
        // 从请求的body中解析出用户名和密码
        const reqBody = JSON.parse(loginRequests[key].body);
        users.push({ username: reqBody.username, password: reqBody.password, token });
      }
    }
  }

  console.log(`==== ✅ K6 Setup: Successfully created ${users.length} users. ====`);
  return { users };
}

// `teardown` 函数在压测结束后执行一次，用于清理测试数据（可选，但推荐）。
export function teardown(data) {
  console.log('==== 🧹 K6 Teardown: Cleaning up... (Optional) ====');
  // 在这里可以添加逻辑，比如调用API删除在setup中创建的用户。
  // console.log(`Teardown completed. Total users created: ${data.users.length}`);
}

// ---------- 场景A: HTTP 压测 ----------
// ===================================================================================
//  сценарий 4. 核心压测场景 (Scenarios) - httpScenario 最终版
// ===================================================================================

/**
 * 模拟HTTP用户的核心业务场景。
 * 该函数由k6的 'http_realistic_scenario' 场景调用。
 * @param {object} data - 由 setup() 函数返回的数据对象，包含了预创建的用户凭证。
 */
export function httpScenario(data) {
  // 1. 安全检查：确保从 setup 阶段成功获取了用户数据
  if (!data || !data.users || data.users.length === 0) {
    console.error('No users available from setup. Skipping iteration.');
    return; // 如果没有可用的用户数据，则提前终止本次迭代
  }

  // 2. VU身份分配：每个虚拟用户(VU)根据其ID，从共享用户列表中获取一个唯一的身份
  const user = data.users[__VU % data.users.length];
  const authToken = user.token;

  // 3. 随机行为模拟：80%的概率成为“内容消费者”，20%的概率成为“内容创作者”
  if (Math.random() < 0.8) {
    // --- 浏览用户的行为流 ---
    group('🧑‍💻 User Flow: Content Consumer (Browser & Liker)', () => {
      // 3.1. 浏览推荐视频流 (Feed)
      const feedRes = http.get(`${BASE_URL}/feed`);
      check(feedRes, { '获取推荐视频流成功 (200)': (r) => r.status === 200 });
      sleep(randomInt(1, 2)); // 模拟用户浏览Feed的思考时间

      // 3.2. 从视频流中随机选择一个视频进行深度交互
      let videoId = 0;
      if (feedRes.status === 200 && feedRes.body) {
        try {
          const feedBody = feedRes.json();
          // 健壮性检查：确保 feedBody.data 是一个非空数组
          if (feedBody && feedBody.data && Array.isArray(feedBody.data) && feedBody.data.length > 0) {
            const videos = feedBody.data;
            const randomVideo = videos[Math.floor(Math.random() * videos.length)];
            videoId = randomVideo.id;
          }
        } catch (e) {
          console.error('Failed to parse feed response JSON');
        }
      }
      
      // 如果从 feed 获取失败（例如feed为空或解析失败），则随机选择一个ID作为后备
      if (!videoId || videoId === 0) {
        videoId = Math.floor(Math.random() * 50) + 1; // 假设视频ID范围为1-50
      }

      // 3.3. 查看视频详情和评论
      group('👀 Viewing Video Details & Comments', () => {
          http.get(`${BASE_URL}/videos/${videoId}`);
          http.get(`${BASE_URL}/videos/${videoId}/comments`);
      });
      sleep(randomInt(1, 2)); // 模拟观看视频和评论的时间

      // 3.4. 50% 的概率进行点赞 -> 取消点赞的完整操作
      if (Math.random() < 0.5) {
        group('👍 Liking & Unliking Flow', () => {
          // 发起点赞请求
          const likeRes = http.post(`${BASE_URL}/videos/${videoId}/like`, null, {
            headers: { 
              'Authorization': `Bearer ${authToken}`,
              'Content-Type': 'application/json',
            },
          });
          check(likeRes, { '点赞成功 (200 or 204)': (r) => r.status === 200 || r.status === 204 });
          
          sleep(randomInt(2, 5)); // 模拟用户点赞后，继续观看一段时间

          // 发起取消点赞请求
          const unlikeRes = http.del(`${BASE_URL}/videos/${videoId}/like`, null, {
            headers: { 'Authorization': `Bearer ${authToken}` },
          });
          check(unlikeRes, { '取消点赞成功 (200 or 204)': (r) => r.status === 200 || r.status === 204 });
        });
      }
    });
  } else {
    // --- 创作用户的行为流 ---
    group('🧑‍🎨 User Flow: Content Creator (Publisher)', () => {
      let createdVideoID = 0;
      const videoPayload = JSON.stringify({
        title: `My K6 Test Video by ${user.username}`,
        description: 'This video was uploaded during a k6 load test.',
        video_url: 'https://example.com/video.mp4',
        cover_url: 'https://example.com/cover.jpg',
      });

      // 3.1. 发布新视频
      const videoRes = http.post(`${BASE_URL}/videos`, videoPayload, {
        headers: { 'Authorization': `Bearer ${authToken}`, 'Content-Type': 'application/json' },
      });
      check(videoRes, { '视频发布成功 (201)': (r) => r.status === 201 });

      if (videoRes.status === 201 && videoRes.body) {
        try {
            createdVideoID = videoRes.json()?.data?.id || 0;
        } catch(e) {
            console.error('Failed to parse create video response JSON');
        }
      }

      // 3.2. 如果视频发布成功，则为其发布一条黄金评论
      if (createdVideoID > 0) {
        sleep(randomInt(1, 2)); // 模拟发布视频后的短暂延迟
        const goldenPayload = JSON.stringify({ content: `Golden comment by ${user.username} on my new video!` });
        const goldenRes = http.post(`${BASE_URL}/videos/${createdVideoID}/golden_comment`, goldenPayload, {
          headers: { 'Authorization': `Bearer ${authToken}`, 'Content-Type': 'application/json' },
        });
        check(goldenRes, { '黄金评论发布成功 (201)': (r) => r.status === 201 });
      }
    });
  }

  // 4. 迭代结束前的全局休眠，模拟用户完成一系列操作后的自然停顿
  sleep(randomInt(1, 3));
}

// ---------- 场景B: WebSocket 压测 ----------
export function wsScenario() {
  const videoId = Math.floor(Math.random() * 50) + 1;
  const url = `${WS_BASE_URL}/${videoId}`;
  const startTime = new Date();

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      // console.log(`VU ${__VU} connected to video room ${videoId}`);
      socket.setInterval(() => {
        socket.send(JSON.stringify({ type: 'heartbeat', timestamp: new Date().getTime() }));
      }, 10000); // 每10秒发送一次心跳
    });

    socket.on('message', (msg) => {
      // console.log(`VU ${__VU} received: ${msg}`);
      wsMessagesReceived.add(1); // 自定义指标：收到的消息数+1
    });

    socket.on('close', () => {
      // console.log(`VU ${__VU} disconnected`);
      const duration = new Date() - startTime;
      wsConnectionDuration.add(duration); // 自定义指标：记录连接总时长
    });

    socket.setTimeout(() => {
      // console.log(`VU ${__VU} closing socket after timeout.`);
      socket.close();
    }, randomInt(5000, 10000)); // 随机在线5-10秒后断开
  });

  check(res, { 'WebSocket 连接成功 (101)': (r) => r && r.status === 101 });
}

// ===================================================================================
// 헬퍼 5. 辅助函数 (Helper Functions)
// ===================================================================================
function randomString(length) {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars[Math.floor(Math.random() * chars.length)];
  }
  return result;
}

function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1) + min);
}
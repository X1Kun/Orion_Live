// attack_script.js - "黄金席位"并发攻击脚本

import http from 'k6/http';
import { check, sleep } from 'k6';

// --- 攻击参数配置 ---
export const options = {
  // 让我们用 500 个并发用户来攻击！
  vus: 500, 
  duration: '30s', 
};

// --- 攻击前准备阶段 (setup) ---
// 这个函数只在测试开始时执行一次，用来获取我们需要的“身份凭证”。
export function setup() {
  // 1. 登录以获取JWT令牌
  const loginRes = http.post('http://localhost:8080/api/v1/users/login', 
    JSON.stringify({
      username: 'testuser', // 假设你已注册了一个名为 'testuser' 的用户
      password: 'password123',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  
  // 从登录响应中提取JWT令牌
  const token = loginRes.json('data.token');
  return { authToken: token }; // 将令牌传递给主攻击阶段
}


// --- 主攻击阶段 (default function) ---
// 每个虚拟用户(VU)会不断重复执行这个函数。
export default function (data) {
  // 目标视频ID (请替换成您自己创建的视频ID)
  const VIDEO_ID = '500'; 

  const url = `http://localhost:8080/api/v1/videos/${VIDEO_ID}/golden_comment`;

  // 模拟每个用户发送不同的评论内容
  const payload = JSON.stringify({
    // `__VU`: k6内置变量，代表当前虚拟用户的ID
    content: `我是攻击者 #${__VU}，来抢黄金席位了!`, 
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      // 在请求头中带上我们的“身份凭证”
      'Authorization': `Bearer ${data.authToken}`, 
    },
  };

  // 发起POST请求，攻击开始！
  const res = http.post(url, payload, params);

  // --- 战果分析 (Checks) ---
  // 检查我们的攻击是否达到了预期的效果
  check(res, {
    '成功抢到席位 (HTTP 201)': (r) => r.status === 201,
    '被系统拒绝 (HTTP 400/500)': (r) => r.status === 400 || r.status === 500,
  });

  // 短暂休息一下，模拟真实用户的间隔
  sleep(1); 
}
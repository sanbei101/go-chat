import ws from 'k6/ws';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '30s',
};

const WS_URL = 'ws://154.8.213.38:8800/ws';
const REQ_HEADERS = {
  'Authorization': 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNTI3MDQ5ZjUtN2FmNS00MWE4LWJkZjMtMjcxZDI2OGQzNDRiIiwiZXhwIjoxNzc1MTk3MTYzLCJpYXQiOjE3NzQ1OTIzNjN9.ID7MkHzqshrXEDWXtUjEUYOMHE8EBbv93HyFwwnmT6I',
};

const message = {
  "client_msg_id": "018e4c50-84a2-7f55-9f1b-2c3d4e5f6a7b",
  "receiver_id": "527049f5-7af5-41a8-bdf3-271d268d344b",
  "chat_type": "single",
  "msg_type": "text",
  "payload": "{\"text\": \"你好\"}",
  "ext": "{}"
};
const TEST_MESSAGE = JSON.stringify(message);

export default function () {
  const res = ws.connect(WS_URL, { headers: REQ_HEADERS }, (socket) => {

    socket.on('open', () => {
      console.log(`[VU ${__VU}] 连接成功`);
      socket.send(TEST_MESSAGE);
      console.log(`[VU ${__VU}] 消息发送完成`);
    });

    socket.on('message', (data) => {
      console.log(`[VU ${__VU}] 收到: ${data}`);
    });

    socket.on('error', (e) => {
      console.error(`[VU ${__VU}] 错误:`, e.error());
    });

    socket.on('close', () => {
      console.log(`[VU ${__VU}] 断开连接`);
    });

    socket.setTimeout(() => {
      socket.close();
    }, 30000);

  });

  check(res, {
    '握手成功 101': (r) => r && r.status === 101,
  });
}
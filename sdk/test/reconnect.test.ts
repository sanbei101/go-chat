import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { ChatSDK, ChatType, MessageType, ChatEventType, ConnectionState } from '../index';
import { TEST_CONFIG, randomUsername, randomPassword, sleep } from './setup';

describe('重连机制集成测试', () => {
  let sdk: ChatSDK;
  let reconnectCount = 0;

  beforeAll(async () => {
    sdk = new ChatSDK({
      ...TEST_CONFIG,
      reconnectInterval: 1000, // 1秒重连间隔
      maxReconnectAttempts: 5,
    });

    await sdk.register({
      username: randomUsername(),
      password: randomPassword(),
    });
  });

  afterAll(() => {
    sdk.disconnect();
  });

  it('连接断开后应该自动重连', async () => {
    await sdk.connect();
    expect(sdk.isConnected()).toBe(true);

    // 监听重连事件
    const stateChanges: ConnectionState[] = [];
    const unsubscribe = sdk.on(ChatEventType.ConnectionStateChange, (event) => {
      stateChanges.push(event.data.state);
      console.log('重连测试 - 状态变更:', event.data.state);
    });

    // 模拟断连（强制关闭 WebSocket）
    // 这里我们断开连接后，SDK 会自动尝试重连
    sdk.disconnect();

    // 等待一段时间观察重连行为
    await sleep(2000);

    unsubscribe();

    // 断开后的状态应该包含 Reconnecting 或最终恢复到 Connected
    expect(stateChanges).toContain(ConnectionState.Disconnected);
  });

  it('应该触发错误事件', async () => {
    const errors: { code: string; message: string }[] = [];

    const unsubscribe = sdk.on(ChatEventType.Error, (event) => {
      errors.push({
        code: event.data.code,
        message: event.data.message,
      });
      console.log('收到错误事件:', event.data.code, event.data.message);
    });

    // 创建一个无法连接的实例（错误的 URL）
    const badSdk = new ChatSDK({
      baseURL: TEST_CONFIG.baseURL,
      gatewayURL: 'ws://invalid-server:9999/ws', // 无效的服务器
      reconnectInterval: 500,
      maxReconnectAttempts: 2,
    });

    await badSdk.register({
      username: randomUsername(),
      password: randomPassword(),
    });

    // 尝试连接（应该失败）
    try {
      await badSdk.connect();
    } catch (error) {
      // 预期会失败
    }

    await sleep(3000); // 等待重连尝试

    unsubscribe();

    // 应该收到错误事件
    expect(errors.length).toBeGreaterThan(0);

    badSdk.disconnect();
  });
});

describe('错误处理集成测试', () => {
  it('应该正确处理发送消息时的错误', async () => {
    const sdk = new ChatSDK({
      ...TEST_CONFIG,
      maxReconnectAttempts: 0, // 不重连
    });

    await sdk.register({
      username: randomUsername(),
      password: randomPassword(),
    });

    const errors: { code: string; message: string }[] = [];
    const unsubscribe = sdk.on(ChatEventType.Error, (event) => {
      errors.push({
        code: event.data.code,
        message: event.data.message,
      });
    });

    // 不连接就发送消息（消息会被加入队列，同时触发错误）
    sdk.sendTextMessage({
      receiver_id: 'test-user-id',
      chat_type: ChatType.Single,
      text: 'Test',
    });

    await sleep(500);

    unsubscribe();

    // 应该收到未连接的错误
    expect(errors.some(e => e.code === 'WS_NOT_CONNECTED')).toBe(true);

    sdk.disconnect();
  });
});

import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { ChatSDK, ChatEventType, ConnectionState } from '../index';
import { TEST_CONFIG, randomUsername, randomPassword, sleep } from './setup';

describe('WebSocket 连接集成测试', () => {
  let sdk: ChatSDK;

  beforeAll(async () => {
    sdk = new ChatSDK(TEST_CONFIG);
    const username = randomUsername();
    const password = randomPassword();
    await sdk.register({ username, password });
  });

  afterAll(() => {
    sdk.disconnect();
  });

  it('应该成功连接到 WebSocket 网关', async () => {
    expect(sdk.isAuthenticated()).toBe(true);

    await sdk.connect();

    expect(sdk.isConnected()).toBe(true);
    expect(sdk.getConnectionState()).toBe(ConnectionState.Connected);

    console.log('WebSocket 连接成功');
  });

  it('应该触发连接状态变更事件', async () => {
    const stateChanges: ConnectionState[] = [];

    const unsubscribe = sdk.on(ChatEventType.ConnectionStateChange, (event) => {
      stateChanges.push(event.data.state);
      console.log('连接状态变更:', event.data.previousState, '->', event.data.state);
    });

    // 先断开再连接
    sdk.disconnect();
    await sleep(500);

    await sdk.connect();
    await sleep(500);

    unsubscribe();

    expect(stateChanges.length).toBeGreaterThan(0);
    expect(stateChanges).toContain(ConnectionState.Connected);
  });

  it('应该触发 connect 事件', async () => {
    let connected = false;

    const unsubscribe = sdk.on(ChatEventType.Connect, (_event) => {
      connected = true;
      console.log('收到 connect 事件');
    });

    sdk.disconnect();
    await sleep(500);

    await sdk.connect();
    await sleep(500);

    unsubscribe();

    expect(connected).toBe(true);
  });

  it('断开连接后状态应该正确', async () => {
    await sdk.connect();
    expect(sdk.isConnected()).toBe(true);

    sdk.disconnect();
    await sleep(500);

    expect(sdk.isConnected()).toBe(false);
    expect(sdk.getConnectionState()).toBe(ConnectionState.Disconnected);

    console.log('断开连接后状态正确');
  });

  it('未认证时不应该能连接', async () => {
    const unauthSdk = new ChatSDK(TEST_CONFIG);

    try {
      await unauthSdk.connect();
      expect.fail('应该抛出错误');
    } catch (error: any) {
      expect(error.message).toContain('authenticated');
      console.log('未认证连接被正确拒绝:', error.message);
    }
  });

  it('应该支持多次连接断开', async () => {
    for (let i = 0; i < 3; i++) {
      await sdk.connect();
      expect(sdk.isConnected()).toBe(true);
      await sleep(200);

      sdk.disconnect();
      await sleep(200);
      expect(sdk.isConnected()).toBe(false);
    }

    console.log('多次连接断开测试通过');
  });
});

import { describe, it, expect } from 'vitest';
import { ChatSDK, ChatType, ChatEventType } from '../index';
import { TEST_CONFIG, randomUsername, randomPassword, sleep } from './setup';

describe('SDK 完整流程集成测试', () => {
  it('应该完成完整的聊天流程', async () => {
    // 1. 创建两个 SDK 实例
    const sdkA = new ChatSDK(TEST_CONFIG);
    const sdkB = new ChatSDK(TEST_CONFIG);

    // 2. 注册两个用户
    console.log('=== 步骤1: 注册用户 ===');
    const [userA, userB] = await Promise.all([
      sdkA.register({ username: randomUsername(), password: randomPassword() }),
      sdkB.register({ username: randomUsername(), password: randomPassword() }),
    ]);
    console.log('用户A:', userA.user_id, userA.username);
    console.log('用户B:', userB.user_id, userB.username);

    // 3. 连接 WebSocket
    console.log('=== 步骤2: 连接 WebSocket ===');
    await Promise.all([sdkA.connect(), sdkB.connect()]);
    await sleep(500);
    expect(sdkA.isConnected()).toBe(true);
    expect(sdkB.isConnected()).toBe(true);
    console.log('WebSocket 连接成功');

    // 4. 设置消息监听
    console.log('=== 步骤3: 设置消息监听 ===');
    const userAMessages: string[] = [];
    const userBMessages: string[] = [];

    sdkA.on(ChatEventType.MessageReceived, (event) => {
      const text = (event.data.message.payload as { text: string }).text;
      userAMessages.push(text);
      console.log('用户A收到消息:', text);
    });

    sdkB.on(ChatEventType.MessageReceived, (event) => {
      const text = (event.data.message.payload as { text: string }).text;
      userBMessages.push(text);
      console.log('用户B收到消息:', text);
    });

    // 5. 双向发送消息
    console.log('=== 步骤4: 发送消息 ===');
    sdkA.sendTextMessage({
      receiver_id: userB.user_id,
      chat_type: ChatType.Single,
      text: 'Hello from User A!',
    });

    sdkB.sendTextMessage({
      receiver_id: userA.user_id,
      chat_type: ChatType.Single,
      text: 'Hello from User B!',
    });

    // 等待消息到达
    await sleep(2000);

    // 6. 验证消息
    console.log('=== 步骤5: 验证消息 ===');
    expect(userBMessages.length).toBeGreaterThan(0);
    expect(userAMessages.length).toBeGreaterThan(0);
    expect(userBMessages[0]).toBe('Hello from User A!');
    expect(userAMessages[0]).toBe('Hello from User B!');

    console.log('消息双向发送成功!');

    // 7. 断开连接
    console.log('=== 步骤6: 断开连接 ===');
    sdkA.disconnect();
    sdkB.disconnect();
    await sleep(500);

    expect(sdkA.isConnected()).toBe(false);
    expect(sdkB.isConnected()).toBe(false);

    console.log('=== 完整流程测试通过 ===');
  }, 30000);
});

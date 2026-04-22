/**
 * 测试配置 - 指向远程测试服务器
 */
export const TEST_CONFIG = {
  /** API 基础 URL */
  baseURL: 'http://154.8.213.38:8801',
  /** WebSocket 网关 URL */
  gatewayURL: 'ws://154.8.213.38:8800/ws',
} as const;

/**
 * 生成随机用户名（用于测试）
 */
export function randomUsername(): string {
  return `test_${Date.now()}_${Math.random().toString(36).substring(2, 8)}`;
}

/**
 * 生成随机密码
 */
export function randomPassword(length = 12): string {
  const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

/**
 * 延迟函数
 */
export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

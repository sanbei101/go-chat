import type {
  ConnectionState,
  ChatEvent,
  ChatEventType,
  EventListener,
  ErrorData,
  ConnectionStateChangeData,
} from './types';

/**
 * 事件发射器 - 用于SDK内部事件管理
 */
export class EventEmitter {
  private listeners: Map<ChatEventType, Set<EventListener>> = new Map();

  /**
   * 监听事件
   */
  on<T>(event: ChatEventType, listener: EventListener<T>): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(listener as EventListener);

    // 返回取消订阅函数
    return () => {
      this.off(event, listener);
    };
  }

  /**
   * 监听一次性事件
   */
  once<T>(event: ChatEventType, listener: EventListener<T>): void {
    const onceWrapper = (e: ChatEvent<T>) => {
      this.off(event, onceWrapper as EventListener);
      listener(e);
    };
    this.on(event, onceWrapper as EventListener);
  }

  /**
   * 取消监听
   */
  off<T>(event: ChatEventType, listener: EventListener<T>): void {
    const set = this.listeners.get(event);
    if (set) {
      set.delete(listener as EventListener);
      if (set.size === 0) {
        this.listeners.delete(event);
      }
    }
  }

  /**
   * 触发事件
   */
  emit<T>(event: ChatEventType, data: T): void {
    const set = this.listeners.get(event);
    if (set) {
      const eventObj: ChatEvent<T> = {
        type: event,
        data,
        timestamp: Date.now(),
      };
      set.forEach((listener) => {
        try {
          listener(eventObj);
        } catch (err) {
          console.error(`Event listener error for ${event}:`, err);
        }
      });
    }
  }

  /**
   * 移除所有监听器
   */
  removeAllListeners(event?: ChatEventType): void {
    if (event) {
      this.listeners.delete(event);
    } else {
      this.listeners.clear();
    }
  }
}

/**
 * 生成UUID v4
 */
export function generateUUID(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

/**
   * 检查字符串是否为有效的UUID
   */
export function isValidUUID(str: string): boolean {
  const uuidRegex =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  return uuidRegex.test(str);
}

/**
 * 创建错误数据对象
 */
export function createError(
  code: string,
  message: string,
  originalError?: Error
): ErrorData {
  return {
    code,
    message,
    originalError,
  };
}

/**
 * 创建连接状态变更数据
 */
export function createStateChange(
  state: ConnectionState,
  previousState: ConnectionState
): ConnectionStateChangeData {
  return {
    state,
    previousState,
  };
}

/**
 * 延迟函数
 */
export function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * 重试函数
 */
export async function retry<T>(
  fn: () => Promise<T>,
  maxAttempts: number,
  delayMs: number,
  shouldRetry?: (error: Error) => boolean
): Promise<T> {
  let lastError: Error;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));

      if (attempt === maxAttempts) {
        throw lastError;
      }

      if (shouldRetry && !shouldRetry(lastError)) {
        throw lastError;
      }

      await delay(delayMs * attempt); // 指数退避
    }
  }

  throw lastError!;
}

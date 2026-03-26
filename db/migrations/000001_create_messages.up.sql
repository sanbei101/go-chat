CREATE TYPE chat_type AS ENUM (
    'single',
    'group',
    'room'
);

CREATE TABLE messages (
    msg_id TEXT PRIMARY KEY,
    client_msg_id TEXT NOT NULL DEFAULT '',
    sender_id TEXT NOT NULL,
    receiver_id TEXT NOT NULL,
    chat_type chat_type NOT NULL,
    server_time BIGINT NOT NULL,
    reply_to_msg_id TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    ext JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_sender_id ON messages (sender_id);
CREATE INDEX idx_messages_receiver_id ON messages (receiver_id);
CREATE INDEX idx_messages_server_time ON messages (server_time DESC);
CREATE INDEX idx_messages_chat_receiver_time ON messages (chat_type, receiver_id, server_time DESC);

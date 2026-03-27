-- Active: 1773747183783@@154.8.213.38@5433@database
CREATE TYPE chat_type AS ENUM (
    'single',
    'group'
);

CREATE TYPE message_type AS ENUM (
    'text',
    'image',
    'video',
    'file'
);

CREATE TABLE messages (
    msg_id uuid PRIMARY KEY,
    client_msg_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    receiver_id uuid NOT NULL,
    chat_type chat_type NOT NULL,
    server_time BIGINT NOT NULL,
    reply_to_msg_id uuid DEFAULT NULL,
    msg_type message_type NOT NULL,
    payload JSONB NOT NULL,
    ext JSONB DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_sender_id ON messages (sender_id);
CREATE INDEX idx_messages_receiver_id ON messages (receiver_id);
CREATE INDEX idx_messages_server_time ON messages (server_time DESC);
CREATE INDEX idx_messages_chat_receiver_time ON messages (chat_type, receiver_id, server_time DESC);

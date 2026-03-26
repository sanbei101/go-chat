-- name: CreateMessage :exec
INSERT INTO messages (
    msg_id,
    client_msg_id,
    sender_id,
    receiver_id,
    chat_type,
    server_time,
    reply_to_msg_id,
    payload,
    ext
) VALUES (
    sqlc.arg(msg_id),
    sqlc.arg(client_msg_id),
    sqlc.arg(sender_id),
    sqlc.arg(receiver_id),
    sqlc.arg(chat_type),
    sqlc.arg(server_time),
    sqlc.arg(reply_to_msg_id),
    sqlc.arg(payload),
    sqlc.arg(ext)
);

-- name: GetMessageByID :one
SELECT
    msg_id,
    client_msg_id,
    sender_id,
    receiver_id,
    chat_type,
    server_time,
    reply_to_msg_id,
    payload,
    ext,
    created_at
FROM messages
WHERE msg_id = sqlc.arg(msg_id)
LIMIT 1;

-- name: ListMessagesByConversation :many
SELECT
    msg_id,
    client_msg_id,
    sender_id,
    receiver_id,
    chat_type,
    server_time,
    reply_to_msg_id,
    payload,
    ext,
    created_at
FROM messages
WHERE receiver_id = sqlc.arg(receiver_id)
  AND chat_type = sqlc.arg(chat_type)
  AND server_time < sqlc.arg(before_server_time)
ORDER BY server_time DESC
LIMIT sqlc.arg(page_size);

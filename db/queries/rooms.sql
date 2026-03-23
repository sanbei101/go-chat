-- name: CreateRoom :one
INSERT INTO room (
    name
) VALUES (
    @name
)
RETURNING id, name, create_date;

-- name: ListRooms :many
SELECT id, name, create_date
FROM room
ORDER BY id ASC;

-- name: RoomMemberExists :one
SELECT EXISTS (
    SELECT 1
    FROM room_member
    WHERE room_id = @room_id
      AND user_id = @user_id
);

-- name: CreateRoomMember :exec
INSERT INTO room_member (
    room_id,
    user_id
) VALUES (
    @room_id,
    @user_id
);

-- name: TouchRoomMember :exec
UPDATE room_member
SET last_online = NOW()
WHERE room_id = @room_id
  AND user_id = @user_id;

-- name: CreateRoomMessage :exec
INSERT INTO room_message (
    room_id,
    user_id,
    message
) VALUES (
    @room_id,
    @user_id,
    @message
);

-- name: ListRecentRoomMessages :many
SELECT
    rm.room_id,
    rm.user_id,
    u.username,
    rm.message,
    rm.created_at
FROM room_message rm
JOIN users u ON rm.user_id = u.id
WHERE rm.room_id = @room_id
  AND rm.created_at >= NOW() - INTERVAL '1 hour'
ORDER BY rm.created_at ASC;

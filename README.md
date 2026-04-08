# sol-core

Backend API for the Sol home automation platform. Built with Go, PostgreSQL (TimescaleDB), Redis, MQTT (VerneMQ), and MinIO.

## Authentication

All endpoints require a Bearer token issued by Keycloak.

```
Authorization: Bearer <access_token>
```

On every authenticated request, the user is automatically upserted into the `users` table from the OIDC claims.

---

## Base URL

```
http://localhost:8080
```

---

## Endpoints

### Health

#### `GET /healthz`

No auth required.

**Response `200`**
```json
{ "status": "ok" }
```

---

### Users

#### `GET /api/v1/me`

Returns the currently authenticated user.

**Response `200`**
```json
{
  "id": "uuid",
  "keycloak_id": "keycloak-subject-id",
  "email": "user@example.com",
  "name": "Jane Doe",
  "created_at": "2026-04-09T10:00:00Z",
  "updated_at": "2026-04-09T10:00:00Z"
}
```

---

### Homes

#### `POST /api/v1/homes`

Create a new home. The caller becomes the owner and is automatically added as a member with role `owner`.

**Request**
```json
{ "name": "My House" }
```

**Response `201`**
```json
{
  "id": "uuid",
  "name": "My House",
  "owner_id": "user-uuid",
  "created_at": "2026-04-09T10:00:00Z",
  "updated_at": "2026-04-09T10:00:00Z"
}
```

---

#### `GET /api/v1/homes`

List all homes the current user is a member of.

**Response `200`**
```json
[
  {
    "id": "uuid",
    "name": "My House",
    "owner_id": "user-uuid",
    "created_at": "2026-04-09T10:00:00Z",
    "updated_at": "2026-04-09T10:00:00Z"
  }
]
```

---

#### `GET /api/v1/homes/{id}`

Get a single home. The caller must be a member.

**Response `200`**
```json
{
  "id": "uuid",
  "name": "My House",
  "owner_id": "user-uuid",
  "created_at": "2026-04-09T10:00:00Z",
  "updated_at": "2026-04-09T10:00:00Z"
}
```

**Errors**
- `403` — not a member of this home
- `404` — home not found

---

#### `PUT /api/v1/homes/{id}`

Update a home's name. Requires role `owner` or `admin`.

**Request**
```json
{ "name": "Beach House" }
```

**Response `200`**
```json
{
  "id": "uuid",
  "name": "Beach House",
  "owner_id": "user-uuid",
  "created_at": "2026-04-09T10:00:00Z",
  "updated_at": "2026-04-09T10:30:00Z"
}
```

**Errors**
- `403` — not owner or admin
- `404` — home not found

---

#### `DELETE /api/v1/homes/{id}`

Delete a home. Requires role `owner`. Cascades to members and invitations.

**Response `204`** — no body

**Errors**
- `403` — not the owner

---

### Home Members

#### `GET /api/v1/homes/{id}/members`

List all members of a home. Caller must be a member.

**Response `200`**
```json
[
  {
    "home_id": "uuid",
    "user_id": "uuid",
    "user_email": "jane@example.com",
    "user_name": "Jane Doe",
    "role": "owner",
    "invited_by": null,
    "joined_at": "2026-04-09T10:00:00Z"
  },
  {
    "home_id": "uuid",
    "user_id": "uuid",
    "user_email": "bob@example.com",
    "user_name": "Bob Smith",
    "role": "member",
    "invited_by": "jane-user-uuid",
    "joined_at": "2026-04-09T11:00:00Z"
  }
]
```

**Errors**
- `403` — not a member of this home

---

#### `POST /api/v1/homes/{id}/members`

Add a user directly by their Sol user ID. Requires role `owner` or `admin`. Role `owner` cannot be assigned.

**Request**
```json
{
  "user_id": "target-user-uuid",
  "role": "member"
}
```

`role` is optional, defaults to `"member"`. Valid values: `"admin"`, `"member"`.

**Response `201`**
```json
{
  "home_id": "uuid",
  "user_id": "uuid",
  "role": "member",
  "invited_by": "actor-user-uuid",
  "joined_at": "2026-04-09T11:00:00Z"
}
```

**Errors**
- `403` — not owner or admin, or attempted to assign `owner` role

---

#### `PATCH /api/v1/homes/{id}/members/{userId}/role`

Change a member's role. Requires role `owner`. Cannot promote to or demote from `owner`.

**Request**
```json
{ "role": "admin" }
```

Valid values: `"admin"`, `"member"`.

**Response `204`** — no body

**Errors**
- `403` — not owner, or trying to change owner's role, or assigning `owner`
- `404` — member not found

---

#### `DELETE /api/v1/homes/{id}/members/{userId}`

Remove a member from a home. Owners cannot be removed. A user can remove themselves (leave). `owner` or `admin` can remove any non-owner member.

**Response `204`** — no body

**Errors**
- `403` — insufficient role, or attempting to remove the owner
- `404` — member not found

---

### Home Invitations

Invitations are email-addressed and expire after 7 days. The invitee's email must match the email on their Sol account when accepting.

#### `POST /api/v1/homes/{id}/invitations`

Invite a user by email. Requires role `owner` or `admin`.

**Request**
```json
{ "email": "newuser@example.com" }
```

**Response `201`**
```json
{
  "id": "uuid",
  "home_id": "uuid",
  "inviter_id": "user-uuid",
  "invitee_email": "newuser@example.com",
  "token": "a3f8c2...(64 hex chars)",
  "status": "pending",
  "expires_at": "2026-04-16T10:00:00Z",
  "created_at": "2026-04-09T10:00:00Z"
}
```

> The `token` is only returned on creation. Share it out-of-band (email, link, etc.).

**Errors**
- `403` — not owner or admin

---

#### `GET /api/v1/homes/{id}/invitations`

List all invitations for a home. Requires role `owner` or `admin`. Tokens are not included in list responses.

**Response `200`**
```json
[
  {
    "id": "uuid",
    "home_id": "uuid",
    "inviter_id": "user-uuid",
    "invitee_email": "newuser@example.com",
    "status": "pending",
    "expires_at": "2026-04-16T10:00:00Z",
    "created_at": "2026-04-09T10:00:00Z"
  }
]
```

**Errors**
- `403` — not owner or admin

---

#### `DELETE /api/v1/homes/{id}/invitations/{invId}`

Cancel a pending invitation. Requires role `owner` or `admin`.

**Response `204`** — no body

**Errors**
- `403` — not owner or admin, or invitation belongs to a different home
- `404` — invitation not found
- `409` — invitation is not in `pending` status

---

#### `POST /api/v1/invitations/{token}/accept`

Accept an invitation. The authenticated user's email must match the invitation's `invitee_email`. The user is added as a `member` of the home.

**No request body.**

**Response `200`**
```json
{
  "id": "uuid",
  "name": "My House",
  "owner_id": "user-uuid",
  "created_at": "2026-04-09T10:00:00Z",
  "updated_at": "2026-04-09T10:00:00Z"
}
```

**Errors**
- `403` — your email does not match the invitation
- `404` — token not found
- `409` — invitation already used or expired

---

#### `POST /api/v1/invitations/{token}/decline`

Decline an invitation. The authenticated user's email must match the invitation's `invitee_email`.

**No request body.**

**Response `204`** — no body

**Errors**
- `403` — your email does not match the invitation
- `404` — token not found
- `409` — invitation already used or expired

---

### Devices

#### `GET /api/v1/devices`

List all devices.

**Response `200`**
```json
[
  {
    "id": "uuid",
    "name": "Living Room Light",
    "type": "light",
    "room_id": "uuid",
    "state": { "brightness": 80, "on": true },
    "metadata": { "manufacturer": "Philips" },
    "firmware_id": "v1.2.0.bin",
    "online": true,
    "created_at": "2026-04-09T10:00:00Z",
    "updated_at": "2026-04-09T10:00:00Z"
  }
]
```

---

#### `POST /api/v1/devices`

Create a device.

**Request**
```json
{
  "name": "Living Room Light",
  "type": "light",
  "room_id": "uuid",
  "metadata": { "manufacturer": "Philips" }
}
```

`type` must be one of: `light`, `switch`, `sensor`, `lock`, `fan`, `custom`.  
`room_id` and `metadata` are optional.

**Response `201`**
```json
{
  "id": "uuid",
  "name": "Living Room Light",
  "type": "light",
  "room_id": "uuid",
  "state": {},
  "online": false,
  "created_at": "2026-04-09T10:00:00Z",
  "updated_at": "2026-04-09T10:00:00Z"
}
```

---

#### `GET /api/v1/devices/{id}`

Get a single device.

**Response `200`** — same shape as list item.

**Errors**
- `404` — device not found

---

#### `PUT /api/v1/devices/{id}`

Update a device. All fields are optional.

**Request**
```json
{
  "name": "Bedroom Light",
  "room_id": "uuid",
  "metadata": { "manufacturer": "IKEA" }
}
```

**Response `200`** — updated device object.

---

#### `DELETE /api/v1/devices/{id}`

Delete a device.

**Response `204`** — no body

---

#### `POST /api/v1/devices/{id}/command`

Send a command to a device via MQTT.

**Request**
```json
{
  "action": "set_brightness",
  "params": { "brightness": 50 }
}
```

**Response `202`** — no body (command is fire-and-forget over MQTT)

---

### Automations

#### `GET /api/v1/automations`

List all automation rules.

**Response `200`**
```json
[
  {
    "id": "uuid",
    "name": "Night mode",
    "description": "Dim lights at 10pm",
    "enabled": true,
    "trigger": {
      "type": "schedule",
      "config": { "cron": "0 22 * * *" }
    },
    "conditions": [
      {
        "type": "time_range",
        "config": { "start": "22:00", "end": "06:00" }
      }
    ],
    "actions": [
      {
        "type": "device_command",
        "config": { "device_id": "uuid", "action": "set_brightness", "params": { "brightness": 10 } }
      }
    ],
    "created_at": "2026-04-09T10:00:00Z",
    "updated_at": "2026-04-09T10:00:00Z"
  }
]
```

---

#### `POST /api/v1/automations`

Create an automation rule.

**Request**
```json
{
  "name": "Night mode",
  "description": "Dim lights at 10pm",
  "trigger": {
    "type": "schedule",
    "config": { "cron": "0 22 * * *" }
  },
  "conditions": [],
  "actions": [
    {
      "type": "device_command",
      "config": { "device_id": "uuid", "action": "set_brightness", "params": { "brightness": 10 } }
    }
  ]
}
```

Trigger types: `device_state`, `schedule`, `event`  
Condition types: `device_state`, `time_range`, `ai_condition`  
Action types: `device_command`, `notification`, `ai_action`

**Response `201`** — created rule object (same shape as list item).

---

#### `GET /api/v1/automations/{id}`

Get a single automation rule.

**Response `200`** — rule object.

**Errors**
- `404` — rule not found

---

#### `PUT /api/v1/automations/{id}`

Update an automation rule. All fields are optional.

**Request**
```json
{
  "name": "Updated name",
  "enabled": false,
  "trigger": { "type": "schedule", "config": { "cron": "0 23 * * *" } }
}
```

**Response `200`** — updated rule object.

---

#### `DELETE /api/v1/automations/{id}`

Delete an automation rule.

**Response `204`** — no body

---

### Firmware

#### `GET /api/v1/firmware`

List all uploaded firmware files stored in MinIO.

**Response `200`**
```json
[
  {
    "name": "v1.2.0.bin",
    "size": 524288,
    "last_modified": "2026-04-09 10:00:00 +0000 UTC"
  }
]
```

---

#### `POST /api/v1/firmware/upload`

Upload a firmware binary. Multipart form, field name `firmware`. Max 100 MB.

**Request** — `Content-Type: multipart/form-data`
```
firmware=@/path/to/firmware.bin
```

**Response `201`**
```json
{ "name": "firmware.bin" }
```

**Errors**
- `400` — missing file or form parse error

---

#### `GET /api/v1/firmware/{id}/download`

Download a firmware file by name (the `id` path param is the filename).

**Response `200`** — binary stream  
`Content-Type: application/octet-stream`  
`Content-Disposition: attachment; filename=<id>`

**Errors**
- `404` — file not found

---

### WebSocket

#### `GET /ws`

Real-time event stream. Requires Bearer token (same as REST).

Upgrade to WebSocket. The server pushes JSON messages when device state changes:

```json
{
  "event": "device.state",
  "data": {
    "device_id": "uuid",
    "state": { "on": true, "brightness": 80 }
  }
}
```

---

## Error Responses

All errors follow this shape:

```json
{ "error": "description" }
```

| Status | Meaning |
|--------|---------|
| `400` | Bad request / missing or invalid body |
| `401` | Missing or invalid Bearer token |
| `403` | Authenticated but not authorized for this action |
| `404` | Resource not found |
| `409` | Conflict (e.g. invitation already used) |
| `500` | Internal server error |

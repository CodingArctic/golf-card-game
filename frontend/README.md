This is the [Next.js](https://nextjs.org) frontend project for Golf Card Game. 

## Pages

### `/` - Home Page
- **Features**: Buttons to Login, Register, and Dashboard (if already authenticated)

### `/login` - User Login
- **Fields**: Username, Password
- **Backend**: `POST /api/login`
- **Success**: Redirects to home page with session cookie
- **Features**: Shows success message when redirected from registration

### `/register` - User Registration
- **Fields**: Username, Email, Password, Confirm Password
- **Backend**: `POST /api/register`
- **Validation**: 
  - Username: minimum 3 characters
  - Email: valid email format
  - Password: minimum 8 characters
  - Confirm Password: must match password
  - Cloudflare Turnstile: must pass before submission
- **Success**: Redirects to login page with success message
- **Error Handling**: Shows clean error messages for duplicate username, nonce failure, and Turnstile token failure.

### `/dash` - Dashboard
- **Fields**: Invite Username, Chat Message
- **Backend**: `POST /api/invite`, `POST /api/logout`, `GET /api/game/list`, `WebSocket /api/ws/chat`
- **Validation**:
  - Invite Username: must exist, game must not be full
  - Chat Message: must not be empty

### `/game?gameId=[id]` - Game Board
- **Fields**: Chat Message
- **Backend**: `WebSocket /api/ws/game/[id]`
- **Validation**:
  - Chat Message: must not be empty
  - Gameplay: invalid moves are blocked on the client-side when not your turn


## Live Development

Only really useful for developing certain non-dynamic pages. Gameplay, chat, and invites are all dependent on WebSockets and/or API Routes handled by Go.

1. Run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

2. Open [http://localhost:3000](http://localhost:3000)

3. Enjoy the hot reloads!

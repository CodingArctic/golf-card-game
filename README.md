# Golf Card Game Online
A project built to recreate the card game [Golf](https://www.wikihow.com/Play-Golf-(Card-Game)), (6 card rules) online. The backend is written in [Go](https://go.dev), accessing a [PostgreSQL](https://www.postgresql.org/) database, and serving a [Next.js](https://nextjs.org/) frontend. The project also uses [Cloudflare Turnstile](https://www.cloudflare.com/application-services/products/turnstile/) as a CAPTCHA replacement, and [Resend](https://resend.com/) for automated emails.

## Setup Instructions
1. Create & populate both `.env` files (one at the root, one in `frontend`)
2. Ensure PostgreSQL is installed, and create a user named `golfer` with 'Log In' permission
3. Build the database: Run the script found in `./ddl/createTables.sql`
4. Build the frontend: `cd frontend`, then `npm run build`
5. Run the Go project: `go run .`


## Task List
- [x] Cloudflare Turnstile
- [x] Registration Welcome Email
- [X] Frontend Mobile Compatibility
- [ ] Built-in Game Tutorial
- [ ] Password Reset
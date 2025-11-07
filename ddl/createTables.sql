DROP DATABASE IF EXISTS golf;

CREATE DATABASE golf;

ALTER DATABASE golf OWNER TO golfer;

\c golf;

CREATE TABLE users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE NOT NULL,
    password TEXT,
    email TEXT
);

CREATE TABLE sessions (
    session_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(user_id),
    token TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ,
    last_active TIMESTAMPTZ DEFAULT now(),
    type TEXT,
    metadata JSONB
);

CREATE TYPE game_status AS ENUM ('waiting_for_players', 'in_progress', 'finished', 'abandoned');

CREATE TABLE games (
    game_id SERIAL PRIMARY KEY,
    public_id UUID DEFAULT gen_random_uuid(),
    created_by UUID REFERENCES users(user_id),
    created_at TIMESTAMPTZ DEFAULT now(),
    status game_status,
    max_players INT,
    player_count INT,
    finished_at TIMESTAMPTZ,
    winner_user_id UUID REFERENCES users(user_id)
);

CREATE TYPE chat_scope AS ENUM ('global', 'game');

CREATE TABLE chat_messages (
    chat_message_id SERIAL PRIMARY KEY,
    sender_user_id UUID REFERENCES users(user_id),
    scope chat_scope,
    game_id INT REFERENCES games(game_id),
    message_text TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE game_players (
    game_player_id SERIAL PRIMARY KEY,
    game_id INT REFERENCES games(game_id),
    user_id UUID REFERENCES users(user_id),
    order_index INT,
    joined_at TIMESTAMPTZ DEFAULT now(),
    left_at TIMESTAMPTZ,
    score INT,
    is_active BOOLEAN
);

CREATE TABLE game_states (
    game_state_id SERIAL PRIMARY KEY,
    game_id INT REFERENCES games(game_id),
    state_json JSONB,
    last_updated TIMESTAMPTZ DEFAULT now(),
    version INT
);

-- change owner to golfer for all tables
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN SELECT tablename FROM pg_tables WHERE schemaname = 'public'
    LOOP
        EXECUTE 'ALTER TABLE ' || quote_ident(r.tablename) || ' OWNER TO golfer;';
    END LOOP;
END$$;

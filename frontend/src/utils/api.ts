interface ApiResponse<T> {
  data?: T;
  error?: string;
}

export interface LoginCredentials {
  username: string;
  password: string;
}

export interface RegisterCredentials {
  username: string;
  email: string;
  password: string;
  nonce?: string;
}

// Simple response structure for custom token authentication
export interface LoginResponse {
  message: string;
}

export interface RegisterResponse {
  message: string;
  user: {
    user_id: string;
    username: string;
    email: string;
  };
}

export interface ApiError {
  error?: string;
  message?: string;
}


/**
 * Makes a type-safe API request to the specified endpoint
 * @param endpoint - The API endpoint to call
 * @param options - Request options including method, body, headers
 * @returns A promise that resolves to the API response
 */
export async function fetchApi<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<ApiResponse<T>> {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api';
  const defaultHeaders: HeadersInit = {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  };

  try {
    const url = `${baseUrl}${endpoint}`;
    
    const response = await fetch(url, {
      ...options,
      headers: {
        ...defaultHeaders,
        ...options.headers,
      },
      credentials: 'include', // Ensure cookies are sent/received
    });

    const data = await response.json();

    // Generic error handling
    if (!response.ok) {
      const errorResponse = data as ApiError;
      return { 
        error: errorResponse.message || errorResponse.error || 'An error occurred'
      };
    }

    return { data: data as T };
  } catch (error) {
    return { error: 'Network error occurred' };
  }
}

/**
 * Authenticates a user with their credentials
 * @param credentials - User login credentials
 * @returns A promise that resolves to the login response with your custom token
 */
export async function loginUser(
  credentials: LoginCredentials
): Promise<ApiResponse<LoginResponse>> {
  // The backend will set a cookie; no need to store token client-side
  return fetchApi<LoginResponse>('/login', {
    method: 'POST',
    body: JSON.stringify(credentials),
  });
}

/**
 * Gets a registration nonce token
 * @returns A promise that resolves to the nonce token
 */
export async function getRegistrationNonce(): Promise<ApiResponse<{ nonce: string }>> {
  return fetchApi<{ nonce: string }>('/register/nonce', {
    method: 'GET',
  });
}

/**
 * Registers a new user account
 * @param credentials - User registration credentials
 * @returns A promise that resolves to the registration response
 */
export async function registerUser(
  credentials: RegisterCredentials
): Promise<ApiResponse<RegisterResponse>> {
  return fetchApi<RegisterResponse>('/register', {
    method: 'POST',
    body: JSON.stringify(credentials),
  });
}

/**
 * Logs out the current user
 * @returns A promise that resolves to the logout response
 */
export async function logoutUser(): Promise<ApiResponse<{ message: string }>> {
  return fetchApi<{ message: string }>('/logout', {
    method: 'POST',
  });
}

// Game API functions

export interface Game {
  gameId: number;
  publicId: string;
  createdBy: string;
  createdAt: string;
  status: string;
  maxPlayers: number;
  playerCount: number;
}

export interface GameInvitation {
  gameId: number;
  publicId: string;
  gamePlayerId: number;
  invitedBy: string;
  invitedByUsername: string;
  createdAt: string;
}

export interface GameListResponse {
  invitations: GameInvitation[];
  activeGames: Game[];
}

/**
 * Creates a new game
 * @returns A promise that resolves to the created game info
 */
export async function createGame(): Promise<ApiResponse<{ gameId: number; publicId: string; status: string }>> {
  return fetchApi<{ gameId: number; publicId: string; status: string }>('/game/create', {
    method: 'POST',
  });
}

/**
 * Invites a player to a game
 * @param gameId - The game ID
 * @param invitedUsername - The username to invite
 * @returns A promise that resolves to the invitation response
 */
export async function invitePlayer(
  gameId: number,
  invitedUsername: string
): Promise<ApiResponse<{ message: string }>> {
  return fetchApi<{ message: string }>('/game/invite', {
    method: 'POST',
    body: JSON.stringify({ gameId, invitedUsername }),
  });
}

/**
 * Accepts a game invitation
 * @param gameId - The game ID to accept
 * @returns A promise that resolves to the acceptance response
 */
export async function acceptInvitation(
  gameId: number
): Promise<ApiResponse<{ message: string }>> {
  return fetchApi<{ message: string }>('/game/accept', {
    method: 'POST',
    body: JSON.stringify({ gameId }),
  });
}

/**
 * Declines a game invitation
 * @param gameId - The game ID to decline
 * @returns A promise that resolves to the decline response
 */
export async function declineInvitation(
  gameId: number
): Promise<ApiResponse<{ message: string }>> {
  return fetchApi<{ message: string }>('/game/decline', {
    method: 'POST',
    body: JSON.stringify({ gameId }),
  });
}

/**
 * Gets list of pending invitations and active games
 * @returns A promise that resolves to the game lists
 */
export async function listGames(): Promise<ApiResponse<GameListResponse>> {
  return fetchApi<GameListResponse>('/game/list', {
    method: 'GET',
  });
}
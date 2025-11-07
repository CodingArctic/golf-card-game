interface ApiResponse<T> {
  data?: T;
  error?: string;
}

export interface LoginCredentials {
  email: string;
  password: string;
}

// Simple response structure for custom token authentication
export interface LoginResponse {
  status: string;
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
    console.log('Making API request to:', url);
    
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

// No client-side token storage or logout needed; rely on backend cookie management
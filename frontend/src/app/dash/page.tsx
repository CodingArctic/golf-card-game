// Icons from Heroicons (https://heroicons.com) - MIT License
"use client";

import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import "../globals.css";
import {
  createGame,
  invitePlayer,
  acceptInvitation,
  declineInvitation,
  listGames,
  logoutUser,
  type GameInvitation,
  type Game,
} from "@/utils/api";

interface Message {
  id: string;
  username: string;
  content: string;
  timestamp: string;
}

export default function DashPage() {
  const router = useRouter();
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputValue, setInputValue] = useState("");
  const [invitations, setInvitations] = useState<GameInvitation[]>([]);
  const [activeGames, setActiveGames] = useState<Game[]>([]);
  const [inviteUsername, setInviteUsername] = useState("");
  const [selectedGameForInvite, setSelectedGameForInvite] = useState<
    number | null
  >(null);
  const [onlinePlayers, setOnlinePlayers] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [isChatOpen, setIsChatOpen] = useState(false);
  const chatEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const shouldReconnectRef = useRef(true);
  const errorTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const successTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const scrollToBottom = (behavior: ScrollBehavior = "smooth") => {
    chatEndRef.current?.scrollIntoView({ behavior });
  };

  // Auto-clear error messages after 5 seconds
  useEffect(() => {
    if (error) {
      if (errorTimeoutRef.current) {
        clearTimeout(errorTimeoutRef.current);
      }
      errorTimeoutRef.current = setTimeout(() => {
        setError("");
      }, 5000);
    }
    return () => {
      if (errorTimeoutRef.current) {
        clearTimeout(errorTimeoutRef.current);
      }
    };
  }, [error]);

  // Auto-clear success messages after 5 seconds
  useEffect(() => {
    if (success) {
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current);
      }
      successTimeoutRef.current = setTimeout(() => {
        setSuccess("");
      }, 5000);
    }
    return () => {
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current);
      }
    };
  }, [success]);

  useEffect(() => {
    // Use a small delay to ensure DOM is fully rendered
    const timeoutId = setTimeout(() => {
      scrollToBottom();
    }, 0);
    return () => clearTimeout(timeoutId);
  }, [messages, isChatOpen, onlinePlayers]);

  useEffect(() => {
    shouldReconnectRef.current = true;
    connectWebSocket();
    loadGames(); // Initial load

    return () => {
      shouldReconnectRef.current = false;
      if (
        wsRef.current &&
        (wsRef.current.readyState === WebSocket.OPEN ||
          wsRef.current.readyState === WebSocket.CONNECTING)
      ) {
        wsRef.current.close(1000, "Navigating away");
        wsRef.current = null;
      }
    };
  }, []);

  const loadGames = async () => {
    const response = await listGames();
    if (response.data) {
      setInvitations(response.data.invitations || []);
      setActiveGames(response.data.activeGames || []);
    }
  };

  const handleCreateGame = async () => {
    setError("");
    setSuccess("");
    const response = await createGame();
    if (response.error) {
      setError(response.error);
    } else if (response.data) {
      setSuccess("Game created successfully!");
      setSelectedGameForInvite(response.data.gameId);
      await loadGames();
    }
  };

  const handleInvite = async () => {
    if (!selectedGameForInvite || !inviteUsername.trim()) {
      setError("Please select a game and enter a username");
      return;
    }

    setError("");
    setSuccess("");
    const response = await invitePlayer(selectedGameForInvite, inviteUsername);
    if (response.error) {
      setError(response.error);
    } else {
      setSuccess(`Invitation sent to ${inviteUsername}!`);
      setInviteUsername("");
    }
  };

  const handleAcceptInvitation = async (gameId: number) => {
    setError("");
    setSuccess("");
    const response = await acceptInvitation(gameId);
    if (response.error) {
      setError(response.error);
    } else {
      setSuccess("Invitation accepted!");
      await loadGames();
      // Navigate to game room
      router.push(`/game?gameId=${gameId}`);
    }
  };

  const handleDeclineInvitation = async (gameId: number) => {
    setError("");
    setSuccess("");
    const response = await declineInvitation(gameId);
    if (response.error) {
      setError(response.error);
    } else {
      setSuccess("Invitation declined");
      await loadGames();
    }
  };

  const handleJoinGame = (gameId: number) => {
    router.push(`/game?gameId=${gameId}`);
  };

  function connectWebSocket() {
    // Don't connect if we're not supposed to reconnect (component unmounted)
    if (!shouldReconnectRef.current) return;

    // Close any existing connection first
    if (wsRef.current) {
      wsRef.current.onclose = null; // Remove handler to prevent reconnection
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws/chat`;

    wsRef.current = new WebSocket(wsUrl);

    wsRef.current.onopen = () => {
      console.log(`Lobby WebSocket has connected`);
    };

    wsRef.current.onerror = (err) => {
      // Only log errors if we're supposed to be connected
      if (shouldReconnectRef.current) {
        console.log(`Lobby WebSocket error occurred: `, err);
      }
    };

    // usually means a network connection failure, but client is still open
    wsRef.current.onclose = (event) => {
      // Only log if not a normal closure and we're still supposed to be connected
      if (shouldReconnectRef.current && event.code !== 1000) {
        console.log(`Lobby WebSocket has disconnected`);
      }

      // only attempt reconnect if component is still mounted
      if (shouldReconnectRef.current) {
        setTimeout(connectWebSocket, 3000);
      }
    };

    wsRef.current.onmessage = (event) => {
      try {
        const lobbyMessage = JSON.parse(event.data);

        if (lobbyMessage.type === "chat") {
          // Handle chat messages
          const rawMessage = lobbyMessage.payload;
          const message: Message = {
            id: Date.now().toString() + Math.random(),
            username: rawMessage.username || "User",
            content: rawMessage.message,
            timestamp: rawMessage.time,
          };
          setMessages((prevMessages) => [...prevMessages, message]);
        } else if (lobbyMessage.type === "player_list") {
          // Handle player list updates
          const playerList = lobbyMessage.payload.players || [];
          setOnlinePlayers(playerList);
        } else if (lobbyMessage.type === "invitation_received") {
          // Handle new invitation received
          const payload = lobbyMessage.payload;
          setSuccess(`${payload.inviterUsername} invited you to join a game!`);
          // Reload games to show the new invitation
          loadGames();
        } else if (lobbyMessage.type === "invitation_accepted") {
          // Handle invitation accepted by another player
          const payload = lobbyMessage.payload;
          setSuccess(`${payload.inviteeUsername} accepted your invitation!`);
          // Reload games and navigate to the game room
          loadGames();
          router.push(`/game?gameId=${payload.gameId}`);
        } else if (lobbyMessage.type === "invitation_declined") {
          // Handle invitation declined
          const payload = lobbyMessage.payload;
          setError(`${payload.inviteeUsername} declined your invitation.`);
          // Reload games to update the list
          loadGames();
        }
      } catch (error) {
        console.error("Failed to parse message:", error);
      }
    };
  }

  const handleSendMessage = () => {
    if (inputValue.trim()) {
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
        console.warn("Lobby WebSocket is not connected");
        return;
      }

      const message = {
        message: inputValue,
        time: new Date().toISOString(),
      };

      wsRef.current.send(JSON.stringify(message));
      setInputValue("");
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  const handleLogout = async () => {
    const response = await logoutUser();
    if (response.error) {
      setError(response.error);
    } else {
      // Close WebSocket connection
      shouldReconnectRef.current = false;
      if (wsRef.current) {
        wsRef.current.close();
      }
      // Redirect to home page
      router.push("/");
    }
  };

  // Credit to Scott Sauyet https://stackoverflow.com/questions/64489395/converting-snake-case-string-to-title-case
  const titleCase = (s: string) =>
    s
      .replace(/^[-_]*(.)/, (_, c) => c.toUpperCase())
      .replace(/[-_]+(.)/g, (_, c) => " " + c.toUpperCase());

  return (
    <div className="flex h-screen">
      <main className="flex-1 p-4 md:p-6 overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-xl md:text-3xl font-bold text-gray-900 dark:text-white">
            Dashboard
          </h1>
          <div className="flex gap-1.5 md:gap-2">
            {/* Mobile Chat Toggle */}
            <button
              onClick={() => setIsChatOpen(!isChatOpen)}
              className="md:hidden p-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
              title="Toggle chat"
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
                />
              </svg>
            </button>
            <button
              onClick={() => router.push('/instructions')}
              className="p-2 md:px-4 md:py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
              title="How to play"
            >
              <svg
                className="w-5 h-5 md:hidden"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <span className="hidden md:inline text-sm">Instructions</span>
            </button>
            <button
              onClick={loadGames}
              className="p-2 md:px-4 md:py-2 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-200 rounded hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
              title="Refresh games and invitations"
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            </button>
            <button
              onClick={handleLogout}
              className="px-3 py-2 md:px-4 md:py-2 bg-red-600 text-white rounded hover:bg-red-700 transition-colors text-sm md:text-base"
              title="Log out"
            >
              <span className="hidden sm:inline">Log Out</span>
              <svg
                className="w-5 h-5 sm:hidden"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                />
              </svg>
            </button>
          </div>
        </div>

        {/* Error/Success Messages */}
        {error && (
          <div className="mb-4 p-4 bg-red-100 dark:bg-red-900/20 border border-red-400 dark:border-red-800 text-red-700 dark:text-red-400 rounded">
            {error}
          </div>
        )}
        {success && (
          <div className="mb-4 p-4 bg-green-100 dark:bg-green-900/20 border border-green-400 dark:border-green-800 text-green-700 dark:text-green-400 rounded">
            {success}
          </div>
        )}

        {/* Create Game Section */}
        <div className="mb-8 bg-white dark:bg-gray-800 rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
            Start New Game
          </h2>
          <button
            onClick={handleCreateGame}
            className="px-6 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
          >
            Create Game
          </button>
        </div>

        {/* Invite Player Section */}
        {activeGames.length > 0 && (
          <div className="mb-8 bg-white dark:bg-gray-800 rounded-lg shadow p-4 md:p-6">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
              Invite Player
            </h2>
            <div className="flex flex-col sm:flex-row gap-3">
              <select
                value={selectedGameForInvite ?? ""}
                onChange={(e) => {
                  const value = e.target.value;
                  setSelectedGameForInvite(value === "" ? null : Number(value));
                }}
                className="w-full sm:w-auto px-3 py-2 bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-white border border-gray-300 dark:border-gray-600 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="">Select a game...</option>
                {activeGames.map((game) => (
                  <option key={game.gameId} value={game.gameId}>
                    Game #{game.gameId} - {titleCase(game.status)} (
                    {game.playerCount}/{game.maxPlayers} players)
                  </option>
                ))}
              </select>
              <input
                type="text"
                value={inviteUsername}
                onChange={(e) => setInviteUsername(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleInvite();
                  }
                }}
                placeholder="Username to invite"
                className="flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-white border border-gray-300 dark:border-gray-600 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <button
                onClick={handleInvite}
                className="px-6 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors whitespace-nowrap"
              >
                Send Invite
              </button>
            </div>
          </div>
        )}

        {/* Pending Invitations */}
        {invitations.length > 0 && (
          <div className="mb-8 bg-white dark:bg-gray-800 rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
              Pending Invitations ({invitations.length})
            </h2>
            <div className="space-y-3">
              {invitations.map((invitation) => (
                <div
                  key={invitation.gameId}
                  className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded"
                >
                  <div>
                    <p className="text-gray-900 dark:text-white font-medium">
                      Game #{invitation.gameId}
                    </p>
                    <p className="text-sm text-gray-600 dark:text-gray-400">
                      Invited by {invitation.invitedByUsername}
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleAcceptInvitation(invitation.gameId)}
                      className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
                    >
                      Accept
                    </button>
                    <button
                      onClick={() => handleDeclineInvitation(invitation.gameId)}
                      className="px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
                    >
                      Decline
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Active Games */}
        {activeGames.length > 0 && (
          <div className="mb-8 bg-white dark:bg-gray-800 rounded-lg shadow p-6">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
              Your Games ({activeGames.length})
            </h2>
            <div className="space-y-3">
              {activeGames.map((game) => (
                <div
                  key={game.gameId}
                  className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded"
                >
                  <div>
                    <p className="text-gray-900 dark:text-white font-medium">
                      Game #{game.gameId}
                    </p>
                    <p className="text-sm text-gray-600 dark:text-gray-400">
                      Status: {titleCase(game.status)} • Players:{" "}
                      {game.playerCount}/{game.maxPlayers}
                    </p>
                  </div>
                  <button
                    onClick={() => handleJoinGame(game.gameId)}
                    className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
                  >
                    Join Game
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}
      </main>

      {/* Chat Sidebar - Hidden on mobile unless toggled, always visible on md+ */}
      <aside
        className={`${isChatOpen ? "fixed inset-0 z-50 h-screen" : "hidden"} md:flex md:flex-col md:w-80 md:max-h-screen bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-800 flex flex-col`}
      >
        {/* Mobile Header with Close Button - Only shown on mobile */}
        {isChatOpen && (
          <div className="md:hidden p-4 border-b border-gray-200 dark:border-gray-800 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
              Chat & Players
            </h2>
            <button
              onClick={() => setIsChatOpen(false)}
              className="p-2 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
              title="Close chat"
            >
              <svg
                className="w-6 h-6"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        )}

        {/* Online Players */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-800">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2 flex items-center gap-2">
            <span className="w-2 h-2 bg-green-500 rounded-full"></span>
            Online ({onlinePlayers.length})
          </h3>
          <div className="space-y-1 max-h-32 overflow-y-auto">
            {onlinePlayers.length === 0 ? (
              <p className="text-xs text-gray-500 dark:text-gray-400">
                No players online
              </p>
            ) : (
              onlinePlayers.map((player, index) => (
                <div
                  key={index}
                  className="text-sm text-gray-800 dark:text-gray-200 px-2 py-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                >
                  {player}
                </div>
              ))
            )}
          </div>
        </div>

        {/* Chat Header */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-800">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Lobby Chat
          </h2>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            {messages.length} messages
          </p>
        </div>

        {/* Messages Container */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {messages.length === 0 ? (
            <div className="text-center text-gray-500 dark:text-gray-400 mt-8">
              <p className="text-sm">No messages yet</p>
              <p className="text-xs mt-1">Start the conversation!</p>
            </div>
          ) : (
            messages.map((message) => (
              <div key={message.id} className="text-sm">
                <div className="flex items-baseline gap-2">
                  <span className="font-semibold text-blue-600 dark:text-blue-400">
                    {message.username}
                  </span>
                  <span className="text-xs text-gray-400 dark:text-gray-500">
                    {new Date(message.timestamp).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </span>
                </div>
                <p className="text-gray-700 dark:text-gray-300 mt-1">
                  {message.content}
                </p>
              </div>
            ))
          )}
          <div ref={chatEndRef} />
        </div>

        {/* Input Area */}
        <div className="p-4 border-t border-gray-200 dark:border-gray-800">
          <div className="flex flex-col gap-2">
            <div className="flex gap-2">
              <input
                type="text"
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                onKeyPress={handleKeyPress}
                placeholder="Type a message..."
                maxLength={500}
                className="flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-800 text-gray-900 dark:text-white rounded border border-gray-300 dark:border-gray-700 focus:border-blue-500 focus:outline-none"
              />
              <button
                onClick={handleSendMessage}
                className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={!inputValue.trim()}
                title="Send message"
              >
                <svg
                  className="w-5 h-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5"
                  />
                </svg>
              </button>
            </div>
            {inputValue && (
              <div className="text-xs text-right">
                <span
                  className={
                    inputValue.length > 450
                      ? "text-orange-500 dark:text-orange-400"
                      : "text-gray-500 dark:text-gray-400"
                  }
                >
                  {inputValue.length}/500
                </span>
              </div>
            )}
          </div>
        </div>
      </aside>
    </div>
  );
}

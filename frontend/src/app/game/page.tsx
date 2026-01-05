"use client";

import { useSearchParams, useRouter } from "next/navigation";
import { useEffect, useState, useRef, Suspense } from "react";
import Card from "@/components/Card";
import {
  createGame,
  invitePlayer,
  acceptInvitation,
  listGames,
  type GameInvitation,
} from "@/utils/api";
import { formatGameId } from "@/utils/gameId";

interface CardData {
  suit: string;
  value: string;
  index: number;
}

interface PlayerInfo {
  userId: string;
  username: string;
  score: number | null;
  isActive: boolean;
  isYou: boolean;
}

interface GameState {
  publicId: string;
  status: string;
  phase: string;
  currentPlayerId: string;
  currentUserId: string;
  currentTurn: number;
  players: PlayerInfo[];
  yourCards: CardData[];
  opponentCards: CardData[];
  drawnCard: CardData | null;
  discardTopCard: CardData | null;
  deckCount: number;
}

interface ChatMessage {
  message: string;
  username: string;
  time: string;
}

interface GameMessage {
  type: string;
  payload: unknown;
}

interface GameEndData {
  winnerUserId: string;
  winnerUsername: string;
  scores: { [userId: string]: number };
}

function GameRoomContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const publicId = searchParams.get("id");

  const [gameState, setGameState] = useState<GameState | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [messageInput, setMessageInput] = useState("");
  const [wsConnected, setWsConnected] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [discardMode, setDiscardMode] = useState(false);
  const [gameEndData, setGameEndData] = useState<GameEndData | null>(null);
  const [gameEndTime, setGameEndTime] = useState<Date | null>(null);
  const [isChatOpen, setIsChatOpen] = useState(false);
  const [rematchRequested, setRematchRequested] = useState(false);
  const [rematchInvitations, setRematchInvitations] = useState<
    GameInvitation[]
  >([]);
  const [rematchLoading, setRematchLoading] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  const checkForRematchInvitations = async () => {
    if (!gameState || !gameEndTime) return;

    const response = await listGames();
    if (response.data?.invitations) {
      // Find invitations from players who were in this game
      const playerUserIds = gameState.players.map((p) => p.userId);
      const rematchInvites = response.data.invitations.filter((inv) => {
        // Check if invitation is from a player in the game
        if (
          !playerUserIds.includes(inv.invitedBy) ||
          inv.invitedBy === gameState.currentUserId
        ) {
          return false;
        }
        // Only include invitations created AFTER the game ended (likely rematch invites)
        const invitationTime = new Date(inv.createdAt);
        return invitationTime > gameEndTime;
      });
      setRematchInvitations(rematchInvites);
    }
  };

  const handleRematch = async () => {
    if (!gameState || rematchLoading) return;

    setRematchLoading(true);
    try {
      // First check for existing rematch invitations
      await checkForRematchInvitations();

      // If there's already a rematch invitation from another player, accept it instead
      if (rematchInvitations.length > 0) {
        const invitation = rematchInvitations[0];
        const acceptResponse = await acceptInvitation(invitation.publicId);
        if (acceptResponse.error) {
          setErrorMessage(acceptResponse.error);
          return;
        }
        setGameEndData(null); // Clear the game over modal
        router.push(`/game?id=${invitation.publicId}`);
        return;
      }

      // Deterministic tie-breaking: add a delay based on userId comparison
      // This prevents both players from creating games simultaneously
      const otherPlayers = gameState.players.filter(
        (p) => p.userId !== gameState.currentUserId,
      );
      if (otherPlayers.length > 0) {
        // If our userId is "less than" the other player's, wait longer
        const shouldWait = otherPlayers.some(
          (p) => gameState.currentUserId < p.userId,
        );
        if (shouldWait) {
          // Wait 1.5 seconds to give the other player time to create and send invite
          await new Promise((resolve) => setTimeout(resolve, 1500));
          // Check again after waiting - use fresh data from API
          const recheckResponse = await listGames();
          if (recheckResponse.data?.invitations) {
            const playerUserIds = gameState.players.map((p) => p.userId);
            const freshRematchInvites = recheckResponse.data.invitations.filter(
              (inv) =>
                playerUserIds.includes(inv.invitedBy) &&
                inv.invitedBy !== gameState.currentUserId,
            );
            if (freshRematchInvites.length > 0) {
              const invitation = freshRematchInvites[0];
              const acceptResponse = await acceptInvitation(invitation.publicId);
              if (acceptResponse.error) {
                setErrorMessage(acceptResponse.error);
                return;
              }
              setGameEndData(null); // Clear the game over modal
              router.push(`/game?id=${invitation.publicId}`);
              return;
            }
          }
        }
      }

      // Create a new game
      const createResponse = await createGame();
      if (createResponse.error || !createResponse.data) {
        setErrorMessage(
          createResponse.error || "Failed to create rematch game",
        );
        return;
      }

      const newPublicId = createResponse.data.publicId;

      // Final check: did someone create a game while we were creating ours?
      // Use fresh data from API, not state
      const finalCheckResponse = await listGames();
      if (finalCheckResponse.data?.invitations) {
        const playerUserIds = gameState.players.map((p) => p.userId);
        const freshRematchInvites = finalCheckResponse.data.invitations.filter(
          (inv) =>
            playerUserIds.includes(inv.invitedBy) &&
            inv.invitedBy !== gameState.currentUserId,
        );
        if (freshRematchInvites.length > 0) {
          // Another player created a game first, join theirs instead
          // Note: We've created a game but haven't invited anyone yet,
          // so it will just be an empty waiting game
          const invitation = freshRematchInvites[0];
          const acceptResponse = await acceptInvitation(invitation.publicId);
          if (acceptResponse.error) {
            setErrorMessage(acceptResponse.error);
            return;
          }
          setGameEndData(null); // Clear the game over modal
          router.push(`/game?id=${invitation.publicId}`);
          return;
        }
      }

      // Invite all other players from the finished game
      const invitePromises = gameState.players
        .filter((player) => player.userId !== gameState.currentUserId)
        .map((player) => invitePlayer(newPublicId, player.username));

      await Promise.all(invitePromises);
      setRematchRequested(true);

      // Navigate to the new game
      setGameEndData(null); // Clear the game over modal
      router.push(`/game?id=${newPublicId}`);
    } catch (error) {
      setErrorMessage("Failed to create rematch");
    } finally {
      setRematchLoading(false);
    }
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // Reset rematch state when publicId changes (new game started)
  useEffect(() => {
    setRematchRequested(false);
    setRematchInvitations([]);
    setRematchLoading(false);
    setGameEndData(null);
    setGameEndTime(null);
    setDiscardMode(false);
    setGameState(null);
    setMessages([]);
  }, [publicId]);

  // Check for rematch invitations when game ends
  useEffect(() => {
    if (gameEndData && gameState) {
      checkForRematchInvitations();
      // Check periodically for new rematch invitations
      const interval = setInterval(checkForRematchInvitations, 3000);
      return () => clearInterval(interval);
    }
  }, [gameEndData, gameState]);

  useEffect(() => {
    if (!publicId) {
      router.push("/dash");
      return;
    }

    let isCleaningUp = false;

    // Close any existing connection first
    if (wsRef.current) {
      wsRef.current.onclose = null;
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws/game/${publicId}`;

    wsRef.current = new WebSocket(wsUrl);

    wsRef.current.onopen = () => {
      if (!isCleaningUp) {
        console.log("Game WebSocket has connected");
        setWsConnected(true);
      }
    };

    wsRef.current.onmessage = (event) => {
      const message: GameMessage = JSON.parse(event.data);

      switch (message.type) {
        case "state":
          const state = message.payload as GameState;
          setGameState(state);
          setErrorMessage(null);
          break;
        case "chat":
          setMessages((prev) => [...prev, message.payload as ChatMessage]);
          break;
        case "error":
          const errorPayload = message.payload as { error: string };
          setErrorMessage(errorPayload.error);
          setTimeout(() => setErrorMessage(null), 5000);
          break;
        case "game_end":
          const endData = message.payload as GameEndData;
          setGameEndData(endData);
          setGameEndTime(new Date()); // Record when the game ended
          break;
        case "player_joined":
          console.log("Player joined:", message.payload);
          break;
        case "player_left":
          console.log("Player left:", message.payload);
          break;
        default:
          console.log("Unknown message type:", message.type);
      }
    };

    wsRef.current.onerror = (error) => {
      if (!isCleaningUp) {
        console.error("Game WebSocket error:", error);
      }
    };

    wsRef.current.onclose = (event) => {
      if (!isCleaningUp && event.code !== 1000) {
        console.log("Game WebSocket has disconnected");
      }
      setWsConnected(false);
    };

    return () => {
      isCleaningUp = true;
      // Close with normal closure code (1000)
      if (
        wsRef.current &&
        (wsRef.current.readyState === WebSocket.OPEN ||
          wsRef.current.readyState === WebSocket.CONNECTING)
      ) {
        wsRef.current.close(1000, "Navigating away");
        wsRef.current = null;
      }
    };
  }, [publicId, router]);

  const sendMessage = () => {
    if (!messageInput.trim() || !wsRef.current) return;

    const message: GameMessage = {
      type: "chat",
      payload: {
        message: messageInput,
      },
    };

    wsRef.current.send(JSON.stringify(message));
    setMessageInput("");
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      sendMessage();
    }
  };

  const sendAction = (action: string, data?: { index?: number }) => {
    if (!wsRef.current) return;

    const message: GameMessage = {
      type: "action",
      payload: {
        action,
        data: data || {},
      },
    };

    wsRef.current.send(JSON.stringify(message));
  };

  const handleCardClick = (cardIndex: number, isOpponent: boolean) => {
    if (!gameState || isOpponent) return;

    // Handle based on phase
    if (gameState.phase === "initial_flip") {
      // Initial flip phase - click to flip cards
      sendAction("initial_flip", { index: cardIndex });
    } else if (
      gameState.phase === "main_game" ||
      gameState.phase === "final_round"
    ) {
      // Main game - can only interact if card drawn and it's your turn
      if (
        gameState.drawnCard &&
        gameState.currentPlayerId === gameState.currentUserId
      ) {
        if (discardMode) {
          // Discard drawn card and flip this card
          sendAction("discard_flip", { index: cardIndex });
          setDiscardMode(false);
        } else {
          // Swap this card with drawn card
          sendAction("swap_card", { index: cardIndex });
        }
      }
    }
  };

  const handleDrawDeck = () => {
    sendAction("draw_deck");
  };

  const handleDrawDiscard = () => {
    sendAction("draw_discard");
  };

  const handleEnterDiscardMode = () => {
    setDiscardMode(true);
  };

  // Credit to Scott Sauyet https://stackoverflow.com/questions/64489395/converting-snake-case-string-to-title-case
  const titleCase = (s: string) =>
    s
      .replace(/^[-_]*(.)/, (_, c) => c.toUpperCase())
      .replace(/[-_]+(.)/g, (_, c) => " " + c.toUpperCase());

  // Reset discard mode when drawn card changes
  useEffect(() => {
    if (!gameState?.drawnCard) {
      setDiscardMode(false);
    }
  }, [gameState?.drawnCard]);

  if (!publicId) {
    return null;
  }

  if (!gameState) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-green-800 to-green-900 flex items-center justify-center">
        <div className="text-white text-2xl">Loading game...</div>
      </div>
    );
  }

  const TurnIndicator = (gameState: GameState, isYourTurn: boolean, isHeader: boolean) => {
    let width = window.innerWidth;

    // check width + location of turn indicator for conditional rendering
    if (width <= 768 && isHeader) {
      return <></>
    }
    if (width > 768 && !isHeader) {
      return <></>
    }

    if (gameState.phase == "initial_flip" ||
      gameState.phase == "waiting" ||
      gameState.phase == "finished") {
      return <></>
    }

    if (isYourTurn) {
      return <p className="text-yellow-300 font-semibold mt-2">⭐ Your Turn</p>
    }

    return (
      <p className="text-white font-semibold mt-2">
        ⏲️ Waiting for {opponent?.username}'s turn
      </p>
    );

  }

  const you = gameState.players.find((p) => p.isYou);
  const opponent = gameState.players.find((p) => !p.isYou);

  const isYourTurn = gameState.currentPlayerId === gameState.currentUserId;
  const hasDrawn = gameState.drawnCard !== null;
  const isWaiting =
    gameState.phase === "waiting" || gameState.status === "waiting_for_players";

  return (
    <div className="min-h-screen bg-gradient-to-br from-green-800 to-green-900 flex">
      {/* Main game area */}
      <div className="flex-1 flex flex-col p-4 md:p-8">
        {/* Header */}
        <div className="mb-6">
          <div className="flex items-center justify-between mb-4">
            <button
              type="button"
              onClick={() => router.push("/dash")}
              className="text-white hover:text-green-200"
            >
              ← Back to Dashboard
            </button>
            {/* Mobile Chat Toggle */}
            <button
              type="button"
              onClick={() => setIsChatOpen(!isChatOpen)}
              className="md:hidden px-3 py-2 bg-white/10 text-white rounded hover:bg-white/20 transition-colors"
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
          </div>
          <h1 className="text-2xl md:text-3xl font-bold text-white">
            Golf Card Game
          </h1>
          <p className="text-green-200">
            {formatGameId(gameState.publicId)} • Status: {titleCase(gameState.status)} •
            Phase: {titleCase(gameState.phase)}
          </p>
          {TurnIndicator(gameState, isYourTurn, true)}
          {errorMessage && (
            <div className="mt-2 p-3 bg-red-500/20 border border-red-500 rounded text-red-200">
              {errorMessage}
            </div>
          )}
        </div>

        {/* Phase instructions */}
        {isWaiting && (
          <div className="mb-4 p-4 bg-yellow-500/20 border border-yellow-400 rounded text-white text-center">
            <p className="font-semibold text-lg">Waiting for Players</p>
            <p className="text-sm text-yellow-200 mt-2">
              {gameState.players.length} / 2 players joined
            </p>
            <p className="text-sm text-yellow-200">
              Game will start when all players have joined
            </p>
          </div>
        )}
        {gameState.phase === "initial_flip" && (
          <div className="mb-4 p-4 bg-blue-500/20 border border-blue-400 rounded text-white">
            <p className="font-semibold">Initial Setup Phase</p>
            <p className="text-sm text-blue-200">
              Click to flip 2 cards: one from the top row and one from the
              bottom row
            </p>
          </div>
        )}

        {/* Main game area - responsive layout */}
        {!isWaiting && (
          <div className="flex-1 flex flex-col md:flex-row gap-4 md:gap-8 items-center justify-center">
            {/* Opponent cards */}
            <div className="w-full md:w-auto px-4 md:px-0">
              <div className="text-white mb-2 md:mb-4">
                <h2 className="text-xl font-semibold">
                  {opponent?.username || "Waiting for opponent..."}
                </h2>
                {opponent?.score !== null && opponent?.score !== undefined && (
                  <p className="text-sm md:text-base text-green-200">
                    Score: {opponent.score}
                  </p>
                )}
              </div>
              <div className="grid grid-cols-3 gap-3 sm:gap-4 md:gap-6 max-w-[400px] sm:max-w-[450px] md:max-w-[550px] mx-auto">
                {gameState.opponentCards.map((card) => (
                  <Card
                    key={card.index}
                    suit={card.suit}
                    value={card.value}
                    onClick={() => handleCardClick(card.index, true)}
                    className="w-full max-w-[120px] md:max-w-[160px] mx-auto"
                  />
                ))}
              </div>
            </div>

            {/* Center - Deck, Discard, and Drawn Card */}
            <div className="flex flex-col gap-4 md:gap-8 items-center relative">
              {/* Mobile Drawn Card Overlay */}
              {gameState.drawnCard && isYourTurn && (
                <div className="md:hidden absolute -top-2 left-1/2 -translate-x-1/2 z-10 bg-gray-900/95 rounded-lg p-4 border-2 border-yellow-400 shadow-lg min-w-[320px]">
                  <div className="flex items-center gap-3">
                    <div className="scale-75 origin-left">
                      <Card
                        suit={gameState.drawnCard.suit}
                        value={gameState.drawnCard.value}
                      />
                    </div>
                    <div className="flex-1">
                      {discardMode ? (
                        <p className="text-yellow-300 text-sm font-semibold">
                          Click a face-down card to flip
                        </p>
                      ) : (
                        <>
                          <p className="text-white text-sm mb-2">
                            Click a card to swap
                          </p>
                          <button
                            type="button"
                            onClick={handleEnterDiscardMode}
                            className="px-3 py-1.5 bg-red-600 text-white rounded hover:bg-red-700 text-sm w-full"
                          >
                            Discard & Flip
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                </div>
              )}

              {/* Deck and Discard */}
              <div className="flex gap-6">
                <div className="text-center">
                  <p className="text-white mb-2">
                    Deck ({gameState.deckCount})
                  </p>
                  <button
                    type="button"
                    onClick={handleDrawDeck}
                    disabled={
                      !isYourTurn ||
                      hasDrawn ||
                      gameState.phase === "initial_flip" ||
                      gameState.phase === "finished"
                    }
                    className="disabled:opacity-50 disabled:cursor-not-allowed hover:scale-105 transition-transform"
                  >
                    <Card suit="back" value="hidden" />
                  </button>
                </div>
                <div className="text-center">
                  <p className="text-white mb-2">Discard</p>
                  {gameState.discardTopCard ? (
                    <button
                      type="button"
                      onClick={handleDrawDiscard}
                      disabled={
                        !isYourTurn ||
                        hasDrawn ||
                        gameState.phase === "initial_flip" ||
                        gameState.phase === "finished"
                      }
                      className="disabled:opacity-50 disabled:cursor-not-allowed hover:scale-105 transition-transform"
                    >
                      <Card
                        suit={gameState.discardTopCard.suit}
                        value={gameState.discardTopCard.value}
                      />
                    </button>
                  ) : (
                    <div className="w-[100px] h-[140px] border-2 border-dashed border-white/30 rounded-lg" />
                  )}
                </div>
              </div>

              {/* Desktop Drawn Card Overlay */}
              {gameState.drawnCard && isYourTurn && (
                <div className="hidden md:block absolute top-0 left-1/2 -translate-x-1/2 z-10 bg-gray-900/95 rounded-lg p-5 border-2 border-yellow-400 shadow-lg min-w-[280px]">
                  <p className="text-white font-semibold mb-3 text-center">
                    Drawn Card
                  </p>
                  <div className="flex flex-col items-center gap-3">
                    <Card
                      suit={gameState.drawnCard.suit}
                      value={gameState.drawnCard.value}
                    />
                    <div className="flex flex-col gap-2 w-full">
                      {discardMode ? (
                        <p className="text-yellow-300 text-sm text-center font-semibold">
                          Click a face-down card to flip
                        </p>
                      ) : (
                        <>
                          <p className="text-white text-sm text-center">
                            Click a card to swap
                          </p>
                          <button
                            type="button"
                            onClick={handleEnterDiscardMode}
                            className="px-3 py-2 bg-red-600 text-white rounded hover:bg-red-700 text-sm w-full"
                          >
                            Discard & Flip
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Your cards */}
            <div className="w-full md:w-auto px-4 md:px-0">
              <div className="text-white mb-2 md:mb-4">
                {TurnIndicator(gameState, isYourTurn, false)}
                <h2 className="text-xl font-semibold">
                  {`${you?.username} (You)`}
                </h2>
                {you?.score !== null && you?.score !== undefined && (
                  <p className="text-sm md:text-base text-green-200">
                    Score: {you.score}
                  </p>
                )}
              </div>

              {/* Your cards */}
              <div className="grid grid-cols-3 gap-3 sm:gap-4 md:gap-6 max-w-[400px] sm:max-w-[450px] md:max-w-[550px] mx-auto">
                {gameState.yourCards.map((card) => (
                  <Card
                    key={card.index}
                    suit={card.suit}
                    value={card.value}
                    onClick={() => handleCardClick(card.index, false)}
                    className="w-full max-w-[120px] md:max-w-[160px] mx-auto"
                  />
                ))}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Game End Modal */}
      {gameEndData && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-lg p-8 max-w-md w-full mx-4 border-2 border-yellow-400">
            <h2 className="text-3xl font-bold text-white text-center mb-6">
              🎉 Game Over!
            </h2>
            <div className="bg-green-700 rounded-lg p-4 mb-6">
              <p className="text-white text-center text-lg">
                <span className="font-semibold">Winner:</span>{" "}
                <span className="text-yellow-300 font-bold">
                  {gameEndData.winnerUsername}
                </span>
              </p>
            </div>
            <div className="bg-gray-700 rounded-lg p-4 mb-6">
              <h3 className="text-white font-semibold mb-3 text-center">
                Final Scores
              </h3>
              <div className="space-y-2">
                {gameState?.players.map((player) => (
                  <div
                    key={player.userId}
                    className="flex justify-between items-center p-2 bg-gray-600 rounded"
                  >
                    <span className="text-white">
                      {player.username}
                      {player.userId === gameEndData.winnerUserId && " 👑"}
                    </span>
                    <span className="text-yellow-300 font-bold">
                      {gameEndData.scores[player.userId]} points
                    </span>
                  </div>
                ))}
              </div>
            </div>
            <div className="space-y-3">
              <button
                type="button"
                onClick={handleRematch}
                disabled={rematchLoading || rematchRequested}
                className={`w-full px-6 py-3 rounded-lg font-semibold transition-colors ${rematchInvitations.length > 0
                  ? "bg-green-600 hover:bg-green-700 text-white animate-pulse"
                  : rematchRequested
                    ? "bg-gray-500 text-gray-300 cursor-not-allowed"
                    : "bg-purple-600 hover:bg-purple-700 text-white"
                  }`}
              >
                {rematchLoading ? (
                  "Creating Rematch..."
                ) : rematchInvitations.length > 0 ? (
                  <>🔄 Accept Rematch ({rematchInvitations.length} waiting!)</>
                ) : rematchRequested ? (
                  "Rematch Requested ✓"
                ) : (
                  "🔄 Rematch"
                )}
              </button>
              <button
                type="button"
                onClick={() => router.push("/dash")}
                className="w-full px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-semibold"
              >
                Back to Dashboard
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Chat sidebar - Hidden on mobile unless toggled, always visible on md+ */}
      <div
        className={`${isChatOpen ? "fixed inset-0 z-50 h-screen" : "hidden"} md:flex md:flex-col md:w-80 md:max-h-screen bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-800 flex flex-col`}
      >
        <div className="p-4 border-b border-gray-200 dark:border-gray-800 flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
              Game Chat
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              {wsConnected ? "Connected" : "Disconnected"}
            </p>
          </div>
          {/* Mobile Close Button */}
          <button
            type="button"
            onClick={() => setIsChatOpen(false)}
            className="md:hidden p-2 text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
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

        {/* Messages */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {messages.map((msg, idx) => (
            <div key={idx} className="text-sm">
              <div className="flex items-baseline gap-2">
                <span className="font-semibold text-blue-600 dark:text-blue-400">
                  {msg.username}
                </span>
                <span className="text-xs text-gray-400 dark:text-gray-500">
                  {new Date(msg.time).toLocaleTimeString()}
                </span>
              </div>
              <p className="text-gray-700 dark:text-gray-300 mt-1">
                {msg.message}
              </p>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <div className="p-4 border-t border-gray-200 dark:border-gray-800">
          <div className="flex flex-col gap-2">
            <div className="flex gap-2">
              <input
                type="text"
                value={messageInput}
                onChange={(e) => setMessageInput(e.target.value)}
                onKeyPress={handleKeyPress}
                placeholder="Type a message..."
                maxLength={500}
                className="flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-800 text-gray-900 dark:text-white rounded border border-gray-300 dark:border-gray-700 focus:border-blue-500 focus:outline-none"
              />
              <button
                type="button"
                onClick={sendMessage}
                className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
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
            {messageInput && (
              <div className="text-xs text-right">
                <span
                  className={
                    messageInput.length > 450
                      ? "text-orange-500 dark:text-orange-400"
                      : "text-gray-500 dark:text-gray-400"
                  }
                >
                  {messageInput.length}/500
                </span>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default function GameRoomPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen bg-gradient-to-br from-green-800 to-green-900 flex items-center justify-center">
          <div className="text-white text-2xl">Loading game...</div>
        </div>
      }
    >
      <GameRoomContent />
    </Suspense>
  );
}

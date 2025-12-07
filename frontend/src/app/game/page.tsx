"use client";

import { useSearchParams, useRouter } from "next/navigation";
import { useEffect, useState, useRef, Suspense } from "react";
import Card from "@/components/Card";

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
	gameId: number;
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
	const gameId = searchParams.get("gameId");

	const [gameState, setGameState] = useState<GameState | null>(null);
	const [messages, setMessages] = useState<ChatMessage[]>([]);
	const [messageInput, setMessageInput] = useState("");
	const [wsConnected, setWsConnected] = useState(false);
	const [errorMessage, setErrorMessage] = useState<string | null>(null);
	const [discardMode, setDiscardMode] = useState(false);
	const [gameEndData, setGameEndData] = useState<GameEndData | null>(null);

	const wsRef = useRef<WebSocket | null>(null);
	const messagesEndRef = useRef<HTMLDivElement>(null);

	const scrollToBottom = () => {
		messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
	};

	useEffect(() => {
		scrollToBottom();
	}, [messages]);

	useEffect(() => {
		if (!gameId) {
			router.push("/dash");
			return;
		}

		// Connect to game WebSocket - same origin as the page
		const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
		const wsUrl = `${protocol}//${window.location.host}/api/ws/game/${gameId}`;

		const ws = new WebSocket(wsUrl);
		wsRef.current = ws;

		ws.onopen = () => {
			console.log("Connected to game WebSocket");
			setWsConnected(true);
		};

		ws.onmessage = (event) => {
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

		ws.onerror = (error) => {
			console.error("WebSocket error:", error);
		};

		ws.onclose = () => {
			console.log("WebSocket closed");
			setWsConnected(false);
		};

		return () => {
			ws.close();
		};
	}, [gameId, router]);

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
		} else if (gameState.phase === "main_game" || gameState.phase === "final_round") {
			// Main game - can only interact if card drawn and it's your turn
			if (gameState.drawnCard && gameState.currentPlayerId === gameState.currentUserId) {
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
		s.replace(/^[-_]*(.)/, (_, c) => c.toUpperCase())
			.replace(/[-_]+(.)/g, (_, c) => ' ' + c.toUpperCase())

	// Reset discard mode when drawn card changes
	useEffect(() => {
		if (!gameState?.drawnCard) {
			setDiscardMode(false);
		}
	}, [gameState?.drawnCard]);

	if (!gameId) {
		return null;
	}

	if (!gameState) {
		return (
			<div className="min-h-screen bg-gradient-to-br from-green-800 to-green-900 flex items-center justify-center">
				<div className="text-white text-2xl">Loading game...</div>
			</div>
		);
	}

	const you = gameState.players.find((p) => p.isYou);
	const opponent = gameState.players.find((p) => !p.isYou);

	const isYourTurn = gameState.currentPlayerId === gameState.currentUserId;
	const hasDrawn = gameState.drawnCard !== null;
	const isWaiting = gameState.phase === "waiting" || gameState.status === "waiting_for_players";

	return (
		<div className="min-h-screen bg-gradient-to-br from-green-800 to-green-900 flex">
			{/* Main game area */}
			<div className="flex-1 flex flex-col p-8">
				{/* Header */}
				<div className="mb-6">
					<button
						type="button"
						onClick={() => router.push("/dash")}
						className="text-white hover:text-green-200 mb-4"
					>
						← Back to Dashboard
					</button>
					<h1 className="text-3xl font-bold text-white">Golf Card Game</h1>
					<p className="text-green-200">
						Game #{gameState.gameId} • Status: {titleCase(gameState.status)} • Phase: {titleCase(gameState.phase)}
					</p>
					{isYourTurn && (
						<p className="text-yellow-300 font-semibold mt-2">
							⭐ Your Turn
						</p>
					)}
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
							Click to flip 2 cards: one from the top row and one from the bottom row
						</p>
					</div>
				)}

				{/* Main game area - horizontal layout */}
				{!isWaiting && (
					<div className="flex-1 flex gap-8 items-center justify-center">
						{/* Left side - Opponent cards */}
						<div>
							<div className="text-white mb-4">
								<h2 className="text-xl font-semibold">
									{opponent?.username || "Waiting for opponent..."}
								</h2>
								{opponent?.score !== null && opponent?.score !== undefined && (
									<p className="text-green-200">Score: {opponent.score}</p>
								)}
							</div>
							<div className="grid grid-cols-3 gap-4 max-w-[340px]">
								{gameState.opponentCards.map((card) => (
									<Card
										key={card.index}
										suit={card.suit}
										value={card.value}
										onClick={() => handleCardClick(card.index, true)}
									/>
								))}
							</div>
						</div>

						{/* Center - Deck, Discard, and Drawn Card */}
						<div className="flex flex-col gap-8 items-center">
							{/* Deck and Discard */}
							<div className="flex gap-6">
								<div className="text-center">
									<p className="text-white mb-2">Deck ({gameState.deckCount})</p>
									<button
										type="button"
										onClick={handleDrawDeck}
										disabled={!isYourTurn || hasDrawn || gameState.phase === "initial_flip" || gameState.phase === "finished"}
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
											disabled={!isYourTurn || hasDrawn || gameState.phase === "initial_flip" || gameState.phase === "finished"}
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

							{/* Drawn card and action controls */}
							{gameState.drawnCard && isYourTurn && (
								<div className="bg-white/10 rounded-lg p-4 border-2 border-yellow-400 max-w-[220px]">
									<p className="text-white font-semibold mb-3 text-center">Drawn Card</p>
									<div className="flex flex-col items-center gap-3">
										<Card
											suit={gameState.drawnCard.suit}
											value={gameState.drawnCard.value}
										/>
										<div className="flex flex-col gap-2 w-full">
											{discardMode ? (
												<p className="text-yellow-300 text-xs text-center font-semibold">
													Click one of your face-down cards to flip it
												</p>
											) : (
												<>
													<p className="text-white text-xs text-center">
														Click a card to swap
													</p>
													<button
														type="button"
														onClick={handleEnterDiscardMode}
														className="px-3 py-2 bg-red-600 text-white rounded hover:bg-red-700 text-xs"
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

						{/* Right side - Your cards */}
						<div>
							<div className="text-white mb-4">
								<h2 className="text-xl font-semibold">
									{`${you?.username} (You)`}
								</h2>
								{you?.score !== null && you?.score !== undefined && (
									<p className="text-green-200">Score: {you.score}</p>
								)}
							</div>

							{/* Your cards */}
							<div className="grid grid-cols-3 gap-4 max-w-[340px]">
								{gameState.yourCards.map((card) => (
									<Card
										key={card.index}
										suit={card.suit}
										value={card.value}
										onClick={() => handleCardClick(card.index, false)}
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
						<button
							type="button"
							onClick={() => router.push("/dash")}
							className="w-full px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 font-semibold"
						>
							Back to Dashboard
						</button>
					</div>
				</div>
			)}

			{/* Chat sidebar */}
			<div className="w-80 bg-gray-900 border-l border-gray-800 flex flex-col">
				<div className="p-4 border-b border-gray-800">
					<h2 className="text-xl font-semibold text-white">Game Chat</h2>
					<p className="text-sm text-gray-400">
						{wsConnected ? "Connected" : "Disconnected"}
					</p>
				</div>

				{/* Messages */}
				<div className="flex-1 overflow-y-auto p-4 space-y-3">
					{messages.map((msg, idx) => (
						<div key={idx} className="text-sm">
							<div className="flex items-baseline gap-2">
								<span className="font-semibold text-blue-400">
									{msg.username}
								</span>
								<span className="text-xs text-gray-500">
									{new Date(msg.time).toLocaleTimeString()}
								</span>
							</div>
							<p className="text-gray-300 mt-1">{msg.message}</p>
						</div>
					))}
					<div ref={messagesEndRef} />
				</div>

				{/* Input */}
				<div className="p-4 border-t border-gray-800">
					<div className="flex gap-2">
						<input
							type="text"
							value={messageInput}
							onChange={(e) => setMessageInput(e.target.value)}
							onKeyPress={handleKeyPress}
							placeholder="Type a message..."
							className="flex-1 px-3 py-2 bg-gray-800 text-white rounded border border-gray-700 focus:border-blue-500 focus:outline-none"
						/>
						<button
							type="button"
							onClick={sendMessage}
							className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
						>
							Send
						</button>
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

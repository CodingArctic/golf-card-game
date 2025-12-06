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
}

interface GameState {
	gameId: number;
	status: string;
	currentTurn: number;
	players: PlayerInfo[];
	yourCards: CardData[];
	opponentCards: CardData[];
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

function GameRoomContent() {
	const searchParams = useSearchParams();
	const router = useRouter();
	const gameId = searchParams.get("gameId");

	const [gameState, setGameState] = useState<GameState | null>(null);
	const [messages, setMessages] = useState<ChatMessage[]>([]);
	const [messageInput, setMessageInput] = useState("");
	const [wsConnected, setWsConnected] = useState(false);

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
					setGameState(message.payload as GameState);
					break;
				case "chat":
					setMessages((prev) => [...prev, message.payload as ChatMessage]);
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

	const handleCardClick = (cardIndex: number, isOpponent: boolean) => {
		console.log(
			`Clicked ${isOpponent ? "opponent" : "your"} card at index ${cardIndex}`,
		);
		// TODO: Send game action to server
	};

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

	const opponent = gameState.players.find(
		(p) => p.userId !== gameState.players[0]?.userId,
	);
	const you = gameState.players[0];

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
						Game #{gameState.gameId} • Status: {gameState.status}
					</p>
				</div>

				{/* Opponent section */}
				<div className="mb-8">
					<div className="text-white mb-4">
						<h2 className="text-xl font-semibold">
							{opponent?.username || "Waiting for opponent..."}
						</h2>
						{opponent?.score !== null && opponent?.score !== undefined && (
							<p className="text-green-200">Score: {opponent.score}</p>
						)}
					</div>

					{/* Opponent's cards */}
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

				{/* Center area - deck and discard pile (placeholder) */}
				<div className="flex-1 flex items-center justify-center gap-8">
					<div className="text-center">
						<p className="text-white mb-2">Deck</p>
						<Card suit="back" value="hidden" />
					</div>
					<div className="text-center">
						<p className="text-white mb-2">Discard</p>
						<div className="w-[100px] h-[140px] border-2 border-dashed border-white/30 rounded-lg" />
					</div>
				</div>

				{/* Your section */}
				<div className="mt-8">
					<div className="text-white mb-4">
						<h2 className="text-xl font-semibold">
							{you?.username || "You"}
						</h2>
						{you?.score !== null && (
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

'use client'

import { useState, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import '../globals.css';
import {
    createGame,
    invitePlayer,
    acceptInvitation,
    declineInvitation,
    listGames,
    type GameInvitation,
    type Game,
} from '@/utils/api';

interface Message {
    id: string;
    username: string;
    content: string;
    timestamp: string;
}

export default function DashPage() {
    const router = useRouter();
    const [messages, setMessages] = useState<Message[]>([]);
    const [inputValue, setInputValue] = useState('');
    const [invitations, setInvitations] = useState<GameInvitation[]>([]);
    const [activeGames, setActiveGames] = useState<Game[]>([]);
    const [inviteUsername, setInviteUsername] = useState('');
    const [selectedGameForInvite, setSelectedGameForInvite] = useState<number | null>(null);
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const chatEndRef = useRef<HTMLDivElement>(null);
    const wsRef = useRef<WebSocket | null>(null);

    const scrollToBottom = () => {
        chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages]);

    useEffect(() => {
        connectWebSocket();
        loadGames();
        
        // Poll for new invitations every 10 seconds
        const pollInterval = setInterval(() => {
            loadGames();
        }, 10000);
        
        return () => {
            if (wsRef.current) {
                wsRef.current.close();
            }
            clearInterval(pollInterval);
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
        setError('');
        setSuccess('');
        const response = await createGame();
        if (response.error) {
            setError(response.error);
        } else if (response.data) {
            setSuccess('Game created successfully!');
            setSelectedGameForInvite(response.data.gameId);
            await loadGames();
        }
    };

    const handleInvite = async () => {
        if (!selectedGameForInvite || !inviteUsername.trim()) {
            setError('Please select a game and enter a username');
            return;
        }

        setError('');
        setSuccess('');
        const response = await invitePlayer(selectedGameForInvite, inviteUsername);
        if (response.error) {
            setError(response.error);
        } else {
            setSuccess(`Invitation sent to ${inviteUsername}!`);
            setInviteUsername('');
        }
    };

    const handleAcceptInvitation = async (gameId: number) => {
        setError('');
        setSuccess('');
        const response = await acceptInvitation(gameId);
        if (response.error) {
            setError(response.error);
        } else {
            setSuccess('Invitation accepted!');
            await loadGames();
            // Navigate to game room
            router.push(`/game?gameId=${gameId}`);
        }
    };

    const handleDeclineInvitation = async (gameId: number) => {
        setError('');
        setSuccess('');
        const response = await declineInvitation(gameId);
        if (response.error) {
            setError(response.error);
        } else {
            setSuccess('Invitation declined');
            await loadGames();
        }
    };

    const handleJoinGame = (gameId: number) => {
        router.push(`/game?gameId=${gameId}`);
    };

    function connectWebSocket() {
        const protocol = window.location.protocol === `https:` ? `wss:` : `ws:`,
            wsUrl = `${protocol}//${window.location.host}/api/ws/chat`;

        wsRef.current = new WebSocket(wsUrl);

        wsRef.current.onopen = () => {
            console.log(`WebSocket has connected`);
        };

        wsRef.current.onerror = (err) => {
            console.log(`WebSocket error occurred: `, err);
        };

        // usually means a network connection failure, but client is still open
        wsRef.current.onclose = () => {
            console.log(`WebSocket has disconnected`);

            // attempt reconnect after 3sec
            setTimeout(connectWebSocket, 3000);
        };

        wsRef.current.onmessage = (event) => {
            try {
                const rawMessage = JSON.parse(event.data);
                // Transform the server's message format to our Message interface
                const message: Message = {
                    id: Date.now().toString() + Math.random(),
                    username: rawMessage.username || 'User',
                    content: rawMessage.message,
                    timestamp: rawMessage.time
                };
                setMessages((prevMessages) => [...prevMessages, message]);
            } catch (error) {
                console.error('Failed to parse message:', error);
            }
        };
    }

    const handleSendMessage = () => {
        if (inputValue.trim()) {
            if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
                console.warn('WebSocket is not connected');
                return;
            }

            const message = {
                message: inputValue,
                time: new Date().toISOString()
            };

            wsRef.current.send(JSON.stringify(message));
            setInputValue('');
        }
    };

    const handleKeyPress = (e: React.KeyboardEvent) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSendMessage();
        }
    };

    // Credit to Scott Sauyet https://stackoverflow.com/questions/64489395/converting-snake-case-string-to-title-case
	const titleCase = (s: string) =>
		s.replace(/^[-_]*(.)/, (_, c) => c.toUpperCase())
			.replace(/[-_]+(.)/g, (_, c) => ' ' + c.toUpperCase())

    return (
        <div className='flex h-screen bg-gray-50 dark:bg-gray-900'>
            <main className='flex-1 p-6 overflow-y-auto'>
                <div className="flex items-center justify-between mb-6">
                    <h1 className="text-3xl font-bold text-gray-900 dark:text-white">Dashboard</h1>
                    <button
                        onClick={loadGames}
                        className="px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-200 rounded hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
                        title="Refresh games and invitations"
                    >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                    </button>
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
                    <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">Start New Game</h2>
                    <button
                        onClick={handleCreateGame}
                        className="px-6 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
                    >
                        Create Game
                    </button>
                </div>

                {/* Invite Player Section */}
                {activeGames.length > 0 && (
                    <div className="mb-8 bg-white dark:bg-gray-800 rounded-lg shadow p-6">
                        <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">Invite Player</h2>
                        <div className="flex gap-3">
                            <select
                                value={selectedGameForInvite ?? ''}
                                onChange={(e) => {
                                    const value = e.target.value;
                                    setSelectedGameForInvite(value === '' ? null : Number(value));
                                }}
                                className="px-3 py-2 bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-white border border-gray-300 dark:border-gray-600 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
                            >
                                <option value="">Select a game...</option>
                                {activeGames.map((game) => (
                                    <option key={game.gameId} value={game.gameId}>
                                        Game #{game.gameId} - {titleCase(game.status)} ({game.playerCount}/{game.maxPlayers} players)
                                    </option>
                                ))}
                            </select>
                            <input
                                type="text"
                                value={inviteUsername}
                                onChange={(e) => setInviteUsername(e.target.value)}
                                placeholder="Username to invite"
                                className="flex-1 px-3 py-2 bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-white border border-gray-300 dark:border-gray-600 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
                            />
                            <button
                                onClick={handleInvite}
                                className="px-6 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
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
                                            Status: {titleCase(game.status)} • Players: {game.playerCount}/{game.maxPlayers}
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

            <aside className="w-80 bg-white dark:bg-gray-800 border-l border-gray-200 dark:border-gray-700 flex flex-col">
                {/* Chat Header */}
                <div className="p-4 border-b border-gray-200 dark:border-gray-700">
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Lobby Chat</h2>
                    <p className="text-sm text-gray-500 dark:text-gray-400">{messages.length} messages</p>
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
                            <div key={message.id} className="group">
                                <div className="flex items-baseline gap-2 mb-1">
                                    <span className="font-medium text-sm text-gray-900 dark:text-white">
                                        {message.username}
                                    </span>
                                    <span className="text-xs text-gray-400 dark:text-gray-500">
                                        {new Date(message.timestamp).toLocaleTimeString([], {
                                            hour: '2-digit',
                                            minute: '2-digit'
                                        })}
                                    </span>
                                </div>
                                <div className="bg-gray-100 dark:bg-gray-700 rounded-lg p-3 text-sm text-gray-800 dark:text-gray-200">
                                    {message.content}
                                </div>
                            </div>
                        ))
                    )}
                    <div ref={chatEndRef} />
                </div>

                {/* Input Area */}
                <div className="p-4 border-t border-gray-200 dark:border-gray-700">
                    <div className="flex gap-2">
                        <input
                            type="text"
                            value={inputValue}
                            onChange={(e) => setInputValue(e.target.value)}
                            onKeyPress={handleKeyPress}
                            placeholder="Type a message..."
                            className="flex-1 px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg 
                                     bg-white dark:bg-gray-700 text-gray-900 dark:text-white 
                                     placeholder-gray-500 dark:placeholder-gray-400
                                     focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                        />
                        <button
                            onClick={handleSendMessage}
                            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg 
                                     transition-colors duration-200 focus:outline-none focus:ring-2 
                                     focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 
                                     disabled:cursor-not-allowed"
                            disabled={!inputValue.trim()}
                            title="Send message"
                        >
                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                                    d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                            </svg>
                        </button>
                    </div>
                </div>
            </aside>
        </div>
    )
}
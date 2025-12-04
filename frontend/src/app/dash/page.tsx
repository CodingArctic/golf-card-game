'use client'

import { useState, useRef, useEffect } from 'react';
import '../globals.css';

interface Message {
    id: string;
    username: string;
    content: string;
    timestamp: string;
}

export default function DashPage() {
    const [messages, setMessages] = useState<Message[]>([]);
    const [inputValue, setInputValue] = useState('');
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
        
        return () => {
            if (wsRef.current) {
                wsRef.current.close();
            }
        };
    }, []);

    function connectWebSocket() {
        const protocol = window.location.protocol === `https:` ? `wss:` : `ws:`,
            wsUrl = `${protocol}//${window.location.host}/api/ws/chat`;

        console.log(`ws url: ${wsUrl}, protocol: ${protocol}`);

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
            console.log('Received message:', event.data);
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

            console.log('Sending message:', message);
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

    return (
        <div className='flex h-screen bg-gray-50 dark:bg-gray-900'>
            <main className='flex-1 p-6'>
                {/* Your main content goes here */}
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
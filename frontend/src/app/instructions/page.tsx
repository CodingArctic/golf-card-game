"use client";

import Card from "@/components/Card";
import { useRouter } from "next/navigation";

export default function Instructions() {
    const router = useRouter();

    return (
        <div className="mx-2 sm:mx-auto my-4 max-w-4xl pb-8">
            {/* Back Button */}
            <button
                onClick={() => router.back()}
                className="mb-4 flex items-center gap-2 text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
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
                        d="M10 19l-7-7m0 0l7-7m-7 7h18"
                    />
                </svg>
                <span className="font-medium">Back</span>
            </button>

            <h1 className="text-4xl sm:text-5xl font-bold mb-6">Golf Card Game - Instructions</h1>
            
            {/* Objective */}
            <section className="mb-8">
                <h2 className="text-3xl font-semibold mb-3">Objective</h2>
                <p className="text-lg mb-2">
                    The goal of Golf is to end up with the <strong>lowest number of points possible</strong>.
                    Strategy and memory are key to success!
                </p>
            </section>

            {/* Setup */}
            <section className="mb-8">
                <h2 className="text-3xl font-semibold mb-3">Setup</h2>
                <ol className="list-decimal list-inside space-y-3 text-lg">
                    <li>Each player starts with a <strong>3x2 grid of face-down cards</strong> (6 cards total).</li>
                    <li>
                        To begin the game, each player chooses <strong>one card from the top row</strong> and{" "}
                        <strong>one card from the bottom row</strong> to flip face-up.
                    </li>
                </ol>
                
                {/* Example Grid */}
                <div className="mt-4 p-4 bg-gray-100 dark:bg-gray-800 rounded-lg">
                    <p className="text-sm font-semibold mb-3">Example starting grid:</p>
                    <div className="flex flex-col items-center gap-2">
                        {/* Top row */}
                        <div className="flex gap-2">
                            <Card suit="back" value="hidden" />
                            <Card suit="hearts" value="7" />
                            <Card suit="back" value="hidden" />
                        </div>
                        {/* Bottom row */}
                        <div className="flex gap-2">
                            <Card suit="spades" value="K" />
                            <Card suit="back" value="hidden" />
                            <Card suit="back" value="hidden" />
                        </div>
                    </div>
                    <p className="text-sm mt-3 text-gray-600 dark:text-gray-400">
                        Here, the player has flipped the 7♥ (top row) and K♠ (bottom row).
                    </p>
                </div>
            </section>

            {/* Gameplay */}
            <section className="mb-8">
                <h2 className="text-3xl font-semibold mb-3">Gameplay</h2>
                <p className="text-lg mb-3">Each turn consists of the following steps:</p>
                
                <div className="space-y-4">
                    {/* Step 1 */}
                    <div className="border-l-4 border-blue-500 pl-4">
                        <h3 className="text-xl font-semibold mb-2">Step 1: Draw a Card</h3>
                        <p className="text-lg mb-2">
                            Choose to draw either:
                        </p>
                        <ul className="list-disc list-inside ml-4 space-y-1">
                            <li>The top card from the <strong>deck</strong> (face-down)</li>
                            <li>The top card from the <strong>discard pile</strong> (face-up)</li>
                        </ul>
                    </div>

                    {/* Step 2 */}
                    <div className="border-l-4 border-green-500 pl-4">
                        <h3 className="text-xl font-semibold mb-2">Step 2: Play or Discard</h3>
                        <p className="text-lg mb-2">After drawing from the deck, you have two options:</p>
                        
                        <div className="ml-4 space-y-3">
                            <div>
                                <p className="font-semibold">Option A: Use the drawn card</p>
                                <p>Swap it with any one of your cards (face-up or face-down). The swapped card goes to the discard pile face-up.</p>
                                
                                {/* Example */}
                                <div className="mt-2 p-3 bg-gray-100 dark:bg-gray-800 rounded">
                                    <p className="text-sm font-semibold mb-2">Example:</p>
                                    <div className="flex flex-col sm:flex-row items-center gap-2 sm:gap-3">
                                        <div className="flex items-center gap-2">
                                            <div>
                                                <p className="text-xs mb-1">Drawn card:</p>
                                                <Card suit="diamonds" value="2" />
                                            </div>
                                            <span className="text-xl sm:text-2xl">→</span>
                                            <div>
                                                <p className="text-xs mb-1">Swap with:</p>
                                                <Card suit="spades" value="K" />
                                            </div>
                                        </div>
                                        <span className="text-xl sm:text-2xl hidden sm:inline">→</span>
                                        <span className="text-xl sm:text-2xl sm:hidden">↓</span>
                                        <div>
                                            <p className="text-xs mb-1">Result:</p>
                                            <Card suit="diamonds" value="2" />
                                        </div>
                                    </div>
                                    <p className="text-sm mt-2 text-gray-600 dark:text-gray-400">
                                        You swap the 2♦ with your K♠, improving your score by 11 points!
                                    </p>
                                </div>
                            </div>
                            
                            <div>
                                <p className="font-semibold">Option B: Discard and flip</p>
                                <p>Discard the drawn card and flip one of your face-down cards instead.</p>
                            </div>
                        </div>
                        
                        <p className="text-sm mt-3 text-gray-600 dark:text-gray-400">
                            <strong>Note:</strong> If you draw from the discard pile, you must use that card (Option A only).
                        </p>
                    </div>
                </div>
            </section>

            {/* Scoring */}
            <section className="mb-8">
                <h2 className="text-3xl font-semibold mb-3">Scoring</h2>
                <p className="text-lg mb-3">Understanding card values is crucial:</p>
                
                <div className="space-y-4">
                    {/* Basic Values */}
                    <div>
                        <h3 className="text-xl font-semibold mb-2">Basic Card Values</h3>
                        <ul className="list-disc list-inside space-y-1 text-lg ml-4">
                            <li><strong>Aces (A):</strong> 1 point</li>
                            <li><strong>Number cards (2-10):</strong> Face value (2 = 2 points, 10 = 10 points, etc.)</li>
                            <li><strong>Face Cards (J, Q, K):</strong> 10 points</li>
                            <li><strong>Jokers:</strong> -2 points (subtract 2 from your total!)</li>
                        </ul>
                    </div>

                    {/* Column Canceling */}
                    <div className="p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
                        <h3 className="text-xl font-semibold mb-2">⭐ Column Canceling Rule</h3>
                        <p className="text-lg mb-3">
                            When two <strong>vertically aligned cards have the same value</strong> (suits don't matter), 
                            they cancel out to <strong>0 points</strong>!
                        </p>
                        
                        {/* Example of canceling */}
                        <div className="mt-3 p-3 bg-white dark:bg-gray-800 rounded">
                            <p className="text-sm font-semibold mb-2">Example 3x2 grid:</p>
                            <div className="flex justify-center">
                                <div className="flex gap-3 sm:gap-6 scale-100 origin-top">
                                    <div className="text-center w-[100px] sm:w-auto">
                                        <p className="text-xs mb-2 h-8 flex items-center justify-center">Column 1<br />(cancels!)</p>
                                        <div className="flex flex-col gap-2">
                                            <Card suit="hearts" value="5" />
                                            <Card suit="clubs" value="5" />
                                        </div>
                                        <p className="text-sm mt-2 font-bold text-green-600 dark:text-green-400">= 0 points</p>
                                    </div>
                                    <div className="text-center w-[100px] sm:w-auto">
                                        <p className="text-xs mb-2 h-8 flex items-center justify-center">Column 2<br />(doesn't cancel)</p>
                                        <div className="flex flex-col gap-2">
                                            <Card suit="diamonds" value="7" />
                                            <Card suit="spades" value="3" />
                                        </div>
                                        <p className="text-sm mt-2">= 10 points</p>
                                    </div>
                                    <div className="text-center w-[100px] sm:w-auto">
                                        <p className="text-xs mb-2 h-8 flex items-center justify-center">Column 3<br />(Jokers cancel!)</p>
                                        <div className="flex flex-col gap-2">
                                            <Card suit="joker" value="Joker" />
                                            <Card suit="joker" value="Joker" />
                                        </div>
                                        <p className="text-sm mt-2 font-bold text-green-600 dark:text-green-400">= 0 points</p>
                                    </div>
                                </div>
                            </div>
                            <p className="text-sm mt-3 text-gray-600 dark:text-gray-400">
                                <strong>Total for this grid: 10 points</strong> (only the middle column counts)
                            </p>
                        </div>

                        <p className="text-sm mt-3 text-gray-600 dark:text-gray-400">
                            <strong>Important:</strong> Even Jokers (-2 points each) cancel to 0 points when vertically aligned!
                        </p>
                    </div>
                </div>
            </section>

            {/* End Game */}
            <section className="mb-8">
                <h2 className="text-3xl font-semibold mb-3">Ending the Game</h2>
                <ol className="list-decimal list-inside space-y-2 text-lg">
                    <li>
                        The game ends when <strong>one player has flipped all their cards face-up</strong>.
                    </li>
                    <li>
                        After this happens, <strong>all other players get one final turn</strong> to improve their score.
                    </li>
                    <li>
                        Once everyone has taken their last turn, <strong>scores are calculated</strong>.
                    </li>
                    <li>
                        The player with the <strong>lowest total score wins</strong>!
                    </li>
                </ol>
            </section>

            {/* Strategy Tips */}
            <section className="mb-8">
                <h2 className="text-3xl font-semibold mb-3">Strategy Tips</h2>
                <ul className="list-disc list-inside space-y-2 text-lg ml-4">
                    <li><strong>Remember card positions:</strong> Keep track of where high-value cards are in your grid.</li>
                    <li><strong>Watch the discard pile:</strong> You can see what cards are available to grab.</li>
                    <li><strong>Create columns:</strong> Try to match cards vertically to cancel them out.</li>
                    <li><strong>Use Jokers wisely:</strong> They're valuable for reducing your score, but even better in matched columns!</li>
                    <li><strong>Timing matters:</strong> Don't flip all your cards too early—other players will get a final turn.</li>
                </ul>
            </section>

            {/* Quick Reference */}
            <section className="mb-8 p-4 bg-gray-100 dark:bg-gray-800 rounded-lg">
                <h2 className="text-2xl font-semibold mb-3">Quick Reference</h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                    <div>
                        <h3 className="font-semibold mb-1">Card Values:</h3>
                        <ul className="space-y-1">
                            <li>Ace = 1</li>
                            <li>2-10 = Face value</li>
                            <li>Face Cards = 10</li>
                            <li>Joker = -2</li>
                        </ul>
                    </div>
                    <div>
                        <h3 className="font-semibold mb-1">Special Rules:</h3>
                        <ul className="space-y-1">
                            <li>✓ Vertical matches = 0 points</li>
                            <li>✓ Jokers cancel when matched</li>
                            <li>✓ Game ends when all cards flipped</li>
                            <li>✓ Lowest score wins</li>
                        </ul>
                    </div>
                </div>
            </section>
        </div>
    )
}
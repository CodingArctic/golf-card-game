"use client";

import { useState, useEffect } from "react";

interface CardProps {
  suit: string; // "hearts", "diamonds", "clubs", "spades", "joker", "back"
  value: string; // "A", "2"-"10", "J", "Q", "K", "Joker", "hidden"
  index?: number;
  onClick?: () => void;
  className?: string;
}

export default function Card({
  suit,
  value,
  onClick,
  className = "",
}: CardProps) {
  // Track displayed state (what's currently shown) vs actual state (what should be shown)
  const [displayedSuit, setDisplayedSuit] = useState(suit);
  const [displayedValue, setDisplayedValue] = useState(value);
  const [isFlipping, setIsFlipping] = useState(false);

  // Detect when card changes and trigger animation
  useEffect(() => {
    const displayedFaceDown =
      displayedSuit === "back" || displayedValue === "hidden";
    const actualFaceDown = suit === "back" || value === "hidden";
    const cardChanged = displayedSuit !== suit || displayedValue !== value;

    // Only animate if the card actually changed
    if (cardChanged) {
      // Check if this is a face-down to face-up transition (or vice versa)
      const isFaceFlip = displayedFaceDown !== actualFaceDown;

      if (isFaceFlip) {
        // Face-down to face-up (or vice versa) - use flip animation
        setIsFlipping(true);

        // Switch content at the midpoint of the animation (when card is rotated 90deg)
        const switchTimer = setTimeout(() => {
          setDisplayedSuit(suit);
          setDisplayedValue(value);
        }, 300); // Half of the 600ms animation

        const endTimer = setTimeout(() => setIsFlipping(false), 600);

        return () => {
          clearTimeout(switchTimer);
          clearTimeout(endTimer);
        };
      } else {
        // Same face state (e.g., face-up to different face-up card) - update immediately, no animation
        setDisplayedSuit(suit);
        setDisplayedValue(value);
      }
    }
  }, [suit, value, displayedSuit, displayedValue]);

  // Use displayed state for rendering
  const displayedFaceDown =
    displayedSuit === "back" || displayedValue === "hidden";
  const displayedIsJoker =
    displayedSuit === "joker" || displayedValue === "Joker";

  // Suit colors
  const suitColor = displayedIsJoker
    ? "#7c3aed"
    : displayedSuit === "hearts" || displayedSuit === "diamonds"
      ? "#dc2626"
      : "#1f2937";

  // Suit symbols
  const suitSymbol: Record<string, string> = {
    hearts: "♥",
    diamonds: "♦",
    clubs: "♣",
    spades: "♠",
    joker: "🃏",
  };

  if (displayedFaceDown) {
    // Face-down card with pattern
    return (
      <div
        className={`cursor-pointer ${className} ${
          isFlipping ? "animate-flip" : ""
        }`}
        onClick={onClick}
        style={{ perspective: "1000px" }}
      >
        <svg width="100" height="140" viewBox="0 0 100 140" className="block">
          {/* Card background */}
          <rect
            x="2"
            y="2"
            width="96"
            height="136"
            rx="8"
            fill="#1e40af"
            stroke="#1e3a8a"
            strokeWidth="2"
          />

          {/* Pattern */}
          <g opacity="0.3">
            {[...Array(5)].map((_, row) =>
              [...Array(4)].map((_, col) => (
                <circle
                  key={`${row}-${col}`}
                  cx={20 + col * 20}
                  cy={20 + row * 30}
                  r="8"
                  fill="#3b82f6"
                />
              )),
            )}
          </g>

          {/* Center logo */}
          <text
            x="50"
            y="75"
            fontSize="24"
            fontWeight="bold"
            fill="#3b82f6"
            textAnchor="middle"
            dominantBaseline="middle"
          >
            ⛳
          </text>
        </svg>
      </div>
    );
  }

  // Face-up card
  return (
    <div
      className={`cursor-pointer ${className} ${
        isFlipping ? "animate-flip" : ""
      }`}
      onClick={onClick}
      style={{ perspective: "1000px" }}
    >
      <svg width="100" height="140" viewBox="0 0 100 140" className="block">
        {/* Card background */}
        <rect
          x="2"
          y="2"
          width="96"
          height="136"
          rx="8"
          fill="white"
          stroke="#e5e7eb"
          strokeWidth="2"
        />

        {/* Top-left value and suit */}
        {!displayedIsJoker ? (
          <>
            <text
              x="12"
              y="22"
              fontSize="16"
              fontWeight="bold"
              fill={suitColor}
              textAnchor="middle"
            >
              {displayedValue}
            </text>
            <text
              x="12"
              y="42"
              fontSize="20"
              fill={suitColor}
              textAnchor="middle"
            >
              {suitSymbol[displayedSuit]}
            </text>
          </>
        ) : (
          <text
            x="12"
            y="22"
            fontSize="24"
            textAnchor="middle"
            dominantBaseline="middle"
          >
            🃏
          </text>
        )}

        {/* Center display */}
        {!displayedIsJoker ? (
          <text
            x="50"
            y="75"
            fontSize="48"
            fill={suitColor}
            textAnchor="middle"
            dominantBaseline="middle"
          >
            {suitSymbol[displayedSuit]}
          </text>
        ) : (
          <>
            <text
              x="50"
              y="60"
              fontSize="48"
              textAnchor="middle"
              dominantBaseline="middle"
            >
              🃏
            </text>
            <text
              x="50"
              y="95"
              fontSize="14"
              fontWeight="bold"
              fill={suitColor}
              textAnchor="middle"
              dominantBaseline="middle"
            >
              JOKER
            </text>
          </>
        )}

        {/* Bottom-right value and suit (rotated) */}
        {!displayedIsJoker ? (
          <g transform="rotate(180, 50, 70)">
            <text
              x="12"
              y="22"
              fontSize="16"
              fontWeight="bold"
              fill={suitColor}
              textAnchor="middle"
            >
              {displayedValue}
            </text>
            <text
              x="12"
              y="42"
              fontSize="20"
              fill={suitColor}
              textAnchor="middle"
            >
              {suitSymbol[displayedSuit]}
            </text>
          </g>
        ) : (
          <g transform="rotate(180, 50, 70)">
            <text
              x="12"
              y="22"
              fontSize="24"
              textAnchor="middle"
              dominantBaseline="middle"
            >
              🃏
            </text>
          </g>
        )}
      </svg>
    </div>
  );
}

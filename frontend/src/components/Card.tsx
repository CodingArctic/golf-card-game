"use client";

interface CardProps {
	suit: string; // "hearts", "diamonds", "clubs", "spades", "back"
	value: string; // "A", "2"-"10", "J", "Q", "K", "hidden"
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
	const isFaceDown = suit === "back" || value === "hidden";

	// Suit colors
	const suitColor =
		suit === "hearts" || suit === "diamonds" ? "#dc2626" : "#1f2937";

	// Suit symbols
	const suitSymbol: Record<string, string> = {
		hearts: "♥",
		diamonds: "♦",
		clubs: "♣",
		spades: "♠",
	};

	if (isFaceDown) {
		// Face-down card with pattern
		return (
			<svg
				width="100"
				height="140"
				viewBox="0 0 100 140"
				className={`cursor-pointer ${className}`}
				onClick={onClick}
			>
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
		);
	}

	// Face-up card
	return (
		<svg
			width="100"
			height="140"
			viewBox="0 0 100 140"
			className={`cursor-pointer ${className}`}
			onClick={onClick}
		>
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
			<text
				x="12"
				y="22"
				fontSize="16"
				fontWeight="bold"
				fill={suitColor}
				textAnchor="middle"
			>
				{value}
			</text>
			<text
				x="12"
				y="42"
				fontSize="20"
				fill={suitColor}
				textAnchor="middle"
			>
				{suitSymbol[suit]}
			</text>

			{/* Center suit symbol */}
			<text
				x="50"
				y="75"
				fontSize="48"
				fill={suitColor}
				textAnchor="middle"
				dominantBaseline="middle"
			>
				{suitSymbol[suit]}
			</text>

			{/* Bottom-right value and suit (rotated) */}
			<g transform="rotate(180, 50, 70)">
				<text
					x="12"
					y="22"
					fontSize="16"
					fontWeight="bold"
					fill={suitColor}
					textAnchor="middle"
				>
					{value}
				</text>
				<text
					x="12"
					y="42"
					fontSize="20"
					fill={suitColor}
					textAnchor="middle"
				>
					{suitSymbol[suit]}
				</text>
			</g>
		</svg>
	);
}

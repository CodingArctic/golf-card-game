"use client";

export default function Instructions() {
    return (
        <div className="m-4">
            <h1 className="text-5xl font-bold">Instructions</h1>
            <h1 className="text-3xl">Setup</h1>
            <ul className="list-disc">
                <li>In 6 card rules, each player starts with 6 cards face down.</li>
                <li>
                    The first step of the game is each person flipping two of their cards.
                    One must be from the top row, the other from the bottom row.
                </li>
            </ul>
        </div>
    )
}
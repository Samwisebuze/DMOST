package com.dmost.domain;

import java.util.HashSet;
import java.util.Set;

/**
 * desc: Represents the common set of Dice in Tabletop Games.
 * Used to model the Domain and standardize constants for this concept.
 */
public enum Dice {
    d2("d2",2),
    d4("d4", 4),
    d6("d6",6),
    d8("d8",8),
    d12("d12", 12),
    d20("d20",20),
    d100("d100",100);

    private Integer sides;
    private String symbol;
    Dice(String symbol, Integer sides) {
        this.symbol = symbol;
        this.sides = sides;
    };

    public Integer getSides() {
        return sides;
    }

    public String getSymbol() {
        return symbol;
    }

    public static Set<String> symbols() {
        Set<String> set = new HashSet<>();
        for (Dice d: Dice.values()) {
            set.add(d.symbol);
        }
        return set;
    }

    public static Integer MIN_FACE_VALUE = 1;
}

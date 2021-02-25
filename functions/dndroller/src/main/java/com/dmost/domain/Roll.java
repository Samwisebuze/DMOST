package com.dmost.domain;

import java.util.*;
import java.util.stream.Collectors;

public class Roll {
    private Map<Dice, List<Integer>> results;
    private RollStrategy rollStrategy;

    private Roll(RollStrategy rollStrategy) {
        this();
        if (rollStrategy != null)
            this.rollStrategy = rollStrategy;
    }

    private Roll() {
        this.rollStrategy = new ThreadLocalRollStrategy();
    }

    public static Roll toss(Map<Dice, Integer> dieToRollMap) {
        final Roll roll = new Roll();
        return toss(roll, dieToRollMap);
    }

    public static Roll toss(Map<Dice, Integer> dieToRollMap, RollStrategy rollStrategy) {
        final Roll roll = new Roll(rollStrategy);
        return toss(roll, dieToRollMap);
    }

    private static Roll toss(Roll roll, Map<Dice, Integer> dieToRollMap) {
        roll.apply(dieToRollMap);
        return roll;
    }

    private void apply(Map<Dice, Integer> dieRolls) {
        results = dieRolls.entrySet()
                .stream()
                .collect(Collectors.toMap(
                        Map.Entry::getKey,
                        entry -> this.rollStrategy.apply(entry.getKey(), entry.getValue())
                ));
    }

    public Map<Dice, List<Integer>> outcome() {
        return results;
    }
}


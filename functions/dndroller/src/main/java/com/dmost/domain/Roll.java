package com.dmost.domain;


import java.util.*;
import java.util.concurrent.ThreadLocalRandom;
import java.util.stream.Collectors;

public class Roll {
    private Map<Dice, List<Integer>> results;

    public static Roll toss(Map<Dice, Integer> dieToRollMap) {
        final Roll roll = new Roll();
        roll.apply(dieToRollMap);
        return roll;
    }

    private static List<Integer> rollDice(Dice dice, Integer rolls) {
        return ThreadLocalRandom.current()
                .ints(rolls, Dice.MIN_FACE_VALUE, dice.getSides() + 1)
                .boxed()
                .collect(Collectors.toList());
    }

    private void apply(Map<Dice, Integer> dieRolls) {
        results = dieRolls.entrySet()
                .stream()
                .collect(Collectors.toMap(Map.Entry::getKey, entry -> rollDice(entry.getKey(), entry.getValue())));
    }

    public Map<Dice, List<Integer>> outcome() {
        return results;
    }
}


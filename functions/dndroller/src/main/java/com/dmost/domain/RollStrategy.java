package com.dmost.domain;

import java.util.List;
import java.util.concurrent.ThreadLocalRandom;
import java.util.stream.Collectors;

public interface RollStrategy {
    List<Integer> apply(Dice dice, Integer rolls);
}

class ThreadLocalRollStrategy implements RollStrategy {
    @Override
    public List<Integer> apply(Dice dice, Integer rolls) {
        return ThreadLocalRandom.current()
                .ints(rolls, Dice.MIN_FACE_VALUE, dice.getSides() + 1)
                .boxed()
                .collect(Collectors.toList());
    }
}

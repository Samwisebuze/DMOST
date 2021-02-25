package com.dmost.domain;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.junit.jupiter.params.provider.ValueSource;

import java.util.HashMap;
import java.util.Map;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

class RollTest {

    @BeforeEach
    void setup(){
    }

    @ParameterizedTest
    @MethodSource("provideValuesForDiceRolls")
    void toss_returns_correctDiceRolls(Dice expected_dice, Integer expected_rolls) {
        Map<Dice, Integer> inputMap = new HashMap<>();
        inputMap.put(expected_dice, expected_rolls);

        final Roll roll = Roll.toss(inputMap);

        assertTrue(roll.outcome().containsKey(expected_dice));
        assertEquals(expected_rolls, roll.outcome().get(expected_dice).size());
    }


    private static Stream<Arguments> provideValuesForDiceRolls() {
        return Stream.of(
                Arguments.of(Dice.d2, 2),
                Arguments.of(Dice.d4, 3),
                Arguments.of(Dice.d6, 6),
                Arguments.of(Dice.d8, 8),
                Arguments.of(Dice.d12, 12),
                Arguments.of(Dice.d20, 20),
                Arguments.of(Dice.d100, 100)
        );
    }
}
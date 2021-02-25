package com.dmost.function;

import com.dmost.domain.Dice;
import com.microsoft.azure.functions.ExecutionContext;
import com.microsoft.azure.functions.HttpMethod;
import com.microsoft.azure.functions.HttpRequestMessage;
import com.microsoft.azure.functions.HttpResponseMessage;
import com.microsoft.azure.functions.HttpStatus;
import com.microsoft.azure.functions.annotation.AuthorizationLevel;
import com.microsoft.azure.functions.annotation.BindingName;
import com.microsoft.azure.functions.annotation.FunctionName;
import com.microsoft.azure.functions.annotation.HttpTrigger;
import org.apache.commons.lang3.math.NumberUtils;

import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

/**
 * Azure Function with HTTP Trigger.
 */
public class DiceRollFunction {
    /**
     * This function listens at endpoint "/api/v1/roll/{diceSet}".
     * 1. curl "{your host}/api/v1/roll/1d20"
     */
    @FunctionName("roll")
    public HttpResponseMessage get_v1(
            @HttpTrigger(
                name = "req",
                methods = {HttpMethod.GET},
                authLevel = AuthorizationLevel.ANONYMOUS,
                route = "v1/roll")
            HttpRequestMessage<String> request,
            final ExecutionContext context
    ) {
        // Parse URI Param
        final Map<String, String> queryParameters = request.getQueryParameters();
        final Map<Dice, Integer> diceRollMap = queryParameters.entrySet().stream()
                .filter(entry -> !Dice.symbols().contains(entry.getKey())) // Must be supported Dice
                .filter(entry -> NumberUtils.isParsable(entry.getValue())) // Must be valid Number of rolls
                .collect(Collectors.toMap(entry -> Dice.valueOf(entry.getKey()), entry -> Integer.parseInt(entry.getValue())));

        return request.createResponseBuilder(HttpStatus.OK).body("Hello").build();
    }

    private static boolean isNumeric(String strNum) {
        if (strNum == null) {
            return false;
        }
        try {
            double d = Double.parseDouble(strNum);
        } catch (NumberFormatException nfe) {
            return false;
        }
        return true;
    }
}

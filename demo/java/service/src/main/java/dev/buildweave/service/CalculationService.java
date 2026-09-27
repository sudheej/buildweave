package dev.buildweave.service;
import dev.buildweave.core.Calculator;
public class CalculationService {
  private final Calculator calculator = new Calculator();
  public int calculate(int value) { return calculator.calculate(value); }
}

package dev.buildweave.app;
import dev.buildweave.service.CalculationService;
public final class Main {
  public static void main(String[] args) {
    if (new CalculationService().calculate(4) <= 0) throw new AssertionError("invalid result");
  }
}

@smoke
Feature: pay
  some prose that is not locked

  Background:
    Given a user

  Scenario: pays
    When they pay
    Then it works

  @slow
  Scenario Outline: refunds
    When they refund <n>
    Then they get <n>

    Examples:
      | n |
      | 1 |
      | 2 |

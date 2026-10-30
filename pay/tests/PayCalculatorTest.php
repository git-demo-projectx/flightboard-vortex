<?php

declare(strict_types=1);

namespace Flightboard\Pay\Tests;

use Flightboard\Pay\InvalidPay;
use Flightboard\Pay\PayCalculator;
use Flightboard\Pay\UnsupportedPayType;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

final class PayCalculatorTest extends TestCase
{
    private PayCalculator $calculator;

    protected function setUp(): void
    {
        $this->calculator = new PayCalculator();
    }

    public function testFixedPayIsTheRate(): void
    {
        $quote = $this->calculator->quote(['type' => 'Fixed', 'rate' => 250]);

        self::assertSame('Fixed', $quote['type']);
        self::assertSame(250.0, $quote['total']);
        self::assertSame('Labor - Fixed pay', $quote['lines'][0]['label']);
    }

    public function testHourlyPayMultipliesRateByHours(): void
    {
        $quote = $this->calculator->quote(['type' => 'Hourly', 'rate' => 65, 'hours' => 4]);

        self::assertSame(260.0, $quote['total']);
        self::assertSame('Labor - Hourly pay (4 hr x $65.00)', $quote['lines'][0]['label']);
    }

    public function testHourlyPayAcceptsFractionalHours(): void
    {
        $quote = $this->calculator->quote(['type' => 'Hourly', 'rate' => 70, 'hours' => 2.5]);

        self::assertSame(175.0, $quote['total']);
        self::assertStringContainsString('2.5 hr', $quote['lines'][0]['label']);
    }

    public function testPerDevicePayMultipliesRateByDevices(): void
    {
        $quote = $this->calculator->quote(['type' => 'PerDevice', 'rate' => 45, 'devices' => 6]);

        self::assertSame(270.0, $quote['total']);
        self::assertSame('Labor - Per device pay (6 x $45.00)', $quote['lines'][0]['label']);
    }

    public function testBlendedPayIsNotSupportedYet(): void
    {
        $this->expectException(UnsupportedPayType::class);

        $this->calculator->quote(['type' => 'Blended', 'rate' => 100]);
    }

    public function testFixedPayWithBonus(): void
    {
        $quote = $this->calculator->quote([
            'type' => 'Fixed',
            'rate' => 100,
            'adjustments' => [
                ['kind' => 'Bonus', 'reason' => 'Weekend call-out', 'amountCents' => 2500,
                    'id' => 1, 'createdAt' => '2026-10-30T00:00:00Z'],
            ],
        ]);

        self::assertSame([
            'type' => 'Fixed',
            'lines' => [
                ['label' => 'Labor - Fixed pay', 'amount' => 100.0],
                ['label' => 'Bonus - Weekend call-out', 'amount' => 25.0],
            ],
            'total' => 125.0,
        ], $quote);
    }

    public function testHourlyPayWithExpenseBonusAndDeduction(): void
    {
        $quote = $this->calculator->quote([
            'type' => 'Hourly', 'rate' => 65.555, 'hours' => 3,
            'expenses' => [['category' => 'Parts', 'amountCents' => 10]],
            'adjustments' => [
                ['kind' => 'Bonus', 'reason' => 'Weekend call-out', 'amountCents' => 20],
                ['kind' => 'Deduction', 'reason' => 'Late arrival', 'amountCents' => 15],
            ],
        ]);

        self::assertSame([
            'type' => 'Hourly',
            'lines' => [
                ['label' => 'Labor - Hourly pay (3 hr x $65.56)', 'amount' => 196.67],
                ['label' => 'Expense - Parts', 'amount' => 0.1],
                ['label' => 'Bonus - Weekend call-out', 'amount' => 0.2],
                ['label' => 'Deduction - Late arrival', 'amount' => -0.15],
            ],
            'total' => 196.82,
        ], $quote);
    }

    #[DataProvider('zeroTotalPay')]
    public function testDeductionCanReduceTotalToZero(array $pay): void
    {
        self::assertSame(0.0, $this->calculator->quote($pay)['total']);
    }

    public static function zeroTotalPay(): array
    {
        return [
            'whole dollars and maximum amount' => [[
                'type' => 'Fixed', 'rate' => 1000,
                'adjustments' => [['kind' => 'Deduction', 'reason' => 'Credit', 'amountCents' => 100000]],
            ]],
            'fractional dollars' => [[
                'type' => 'Fixed', 'rate' => 0.3,
                'adjustments' => [
                    ['kind' => 'Deduction', 'reason' => 'Credit', 'amountCents' => 10],
                    ['kind' => 'Deduction', 'reason' => 'Credit', 'amountCents' => 20],
                ],
            ]],
        ];
    }

    public function testRejectsNegativeTotal(): void
    {
        $this->expectException(InvalidPay::class);
        $this->expectExceptionMessage('deductions must not exceed the rest of the pay');

        $this->calculator->quote([
            'type' => 'Fixed', 'rate' => 100,
            'adjustments' => [['kind' => 'Deduction', 'reason' => 'Credit', 'amountCents' => 10001]],
        ]);
    }

    public function testEmptyAdjustmentsPreserveQuote(): void
    {
        $pay = ['type' => 'Fixed', 'rate' => 100.123];

        self::assertSame($this->calculator->quote($pay), $this->calculator->quote($pay + ['adjustments' => []]));
    }

    public function testFiveAdjustmentsAndMinimumAmountAreAllowed(): void
    {
        $quote = $this->calculator->quote([
            'type' => 'Fixed', 'rate' => 100.123,
            'adjustments' => array_fill(0, 5, ['kind' => 'Bonus', 'reason' => 'Credit', 'amountCents' => 1]),
        ]);

        self::assertCount(6, $quote['lines']);
        self::assertSame(['label' => 'Bonus - Credit', 'amount' => 0.01], $quote['lines'][1]);
        self::assertSame(100.17, $quote['total']);
    }

    #[DataProvider('invalidAdjustments')]
    public function testRejectsInvalidAdjustments(mixed $adjustments, string $error): void
    {
        $this->expectException(InvalidPay::class);
        $this->expectExceptionMessage($error);

        $this->calculator->quote(['type' => 'Fixed', 'rate' => 100, 'adjustments' => $adjustments]);
    }

    /** @return array<string, array{0: mixed, 1: string}> */
    public static function invalidAdjustments(): array
    {
        $listError = 'adjustments must be a list';
        $countError = 'adjustments must have at most 5 items';
        $kindError = 'adjustment kind must be Bonus or Deduction';
        $reasonError = 'adjustment reason must be a non-empty string';
        $amountError = 'adjustment amountCents must be a whole number from 1 to 100000';
        $valid = ['kind' => 'Bonus', 'reason' => 'Credit', 'amountCents' => 100];

        return [
            'null list' => [null, $listError],
            'string list' => ['Bonus', $listError],
            'boolean list' => [true, $listError],
            'object list' => [$valid, $listError],
            'empty object list' => [new \stdClass(), $listError],
            'numeric-key object list' => [(object) [$valid], $listError],
            'sparse list' => [[1 => $valid], $listError],
            'count before item validation' => [array_fill(0, 6, null), $countError],
            'missing kind' => [[[]], $kindError],
            'unknown kind before reason' => [[['kind' => 'Other']], $kindError],
            'lowercase kind' => [[array_replace($valid, ['kind' => 'bonus'])], $kindError],
            'non-string kind' => [[array_replace($valid, ['kind' => 1])], $kindError],
            'scalar item' => [[1], $kindError],
            'null item' => [[null], $kindError],
            'missing reason before amount' => [[['kind' => 'Bonus']], $reasonError],
            'empty reason' => [[array_replace($valid, ['reason' => ''])], $reasonError],
            'blank reason' => [[array_replace($valid, ['reason' => " \t\n"])], $reasonError],
            'non-string reason' => [[array_replace($valid, ['reason' => 1])], $reasonError],
            'missing amount' => [[['kind' => 'Bonus', 'reason' => 'Credit']], $amountError],
            'zero amount' => [[array_replace($valid, ['amountCents' => 0])], $amountError],
            'negative amount' => [[array_replace($valid, ['amountCents' => -1])], $amountError],
            'over maximum amount' => [[array_replace($valid, ['amountCents' => 100001])], $amountError],
            'fractional amount' => [[array_replace($valid, ['amountCents' => 1.5])], $amountError],
            'float amount' => [[array_replace($valid, ['amountCents' => 100.0])], $amountError],
            'string amount' => [[array_replace($valid, ['amountCents' => '100'])], $amountError],
            'boolean amount' => [[array_replace($valid, ['amountCents' => true])], $amountError],
            'null amount' => [[array_replace($valid, ['amountCents' => null])], $amountError],
            'input order' => [[['kind' => 'Bonus'], ['kind' => 'Other']], $reasonError],
        ];
    }

    /** @return array<string, array{0: array<string, mixed>}> */
    public static function invalidPay(): array
    {
        return [
            'missing type' => [['rate' => 10]],
            'unknown type' => [['type' => 'Barter', 'rate' => 10]],
            'missing rate' => [['type' => 'Fixed']],
            'string rate' => [['type' => 'Fixed', 'rate' => '10']],
            'zero rate' => [['type' => 'Fixed', 'rate' => 0]],
            'hourly without hours' => [['type' => 'Hourly', 'rate' => 10]],
            'negative devices' => [['type' => 'PerDevice', 'rate' => 10, 'devices' => -1]],
        ];
    }

    /** @param array<string, mixed> $pay */
    #[DataProvider('invalidPay')]
    public function testRejectsInvalidPay(array $pay): void
    {
        $this->expectException(InvalidPay::class);

        $this->calculator->quote($pay);
    }
}

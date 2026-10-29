<?php

declare(strict_types=1);

namespace Flightboard\Pay;

/**
 * HTTP routing for the pay service, kept free of globals so tests can call it.
 *
 * Routes:
 *   GET  /health  -> {"status":"ok"}
 *   POST /quote   -> a quote from PayCalculator::quote()
 *   POST /quotes  -> batch labor and approved expense totals
 */
final class App
{
    public const MAX_BODY_BYTES = 65536;
    private const MAX_JSON_DEPTH = 32;

    public function __construct(private readonly PayCalculator $calculator = new PayCalculator())
    {
    }

    /** @return array{0: int, 1: array<string, mixed>} status code and JSON body */
    public function handle(string $method, string $path, string $body): array
    {
        $route = $method . ' ' . rtrim($path, '/');

        if (in_array($route, ['POST /quote', 'POST /quotes'], true) && strlen($body) > self::MAX_BODY_BYTES) {
            return [413, ['error' => 'request body must not exceed ' . self::MAX_BODY_BYTES . ' bytes']];
        }

        return match ($route) {
            'GET /health' => [200, ['status' => 'ok']],
            'POST /quote' => $this->quote($body),
            'POST /quotes' => $this->quotes($body),
            default => [404, ['error' => 'not found']],
        };
    }

    /** @return array{0: int, 1: array<string, mixed>} */
    private function quotes(string $body): array
    {
        try {
            $decoded = $this->decodeBody($body);

            // Associative decoding loses the distinction between objects and lists.
            $items = json_decode($body, false, self::MAX_JSON_DEPTH)->items ?? null;
            if (is_object($items)) {
                $decoded['items'] = $items;
            }

            return [200, (new BatchQuoter($this->calculator))->quote($decoded['items'] ?? null)];
        } catch (InvalidPay $e) {
            return [400, ['error' => $e->getMessage()]];
        } catch (UnsupportedPayType $e) {
            return [422, ['error' => $e->getMessage()]];
        }
    }

    /** @return array{0: int, 1: array<string, mixed>} */
    private function quote(string $body): array
    {
        try {
            $pay = $this->decodeBody($body);

            // Associative decoding loses the distinction between objects and lists.
            $expenses = json_decode($body, false, self::MAX_JSON_DEPTH)->expenses ?? null;
            if (is_object($expenses)) {
                $pay['expenses'] = $expenses;
            }

            return [200, $this->calculator->quote($pay)];
        } catch (InvalidPay $e) {
            return [400, ['error' => $e->getMessage()]];
        } catch (UnsupportedPayType $e) {
            return [422, ['error' => $e->getMessage()]];
        }
    }

    /** @return array<array-key, mixed> */
    private function decodeBody(string $body): array
    {
        $decoded = json_decode($body, true, self::MAX_JSON_DEPTH);
        if (json_last_error() === JSON_ERROR_DEPTH) {
            throw new InvalidPay('JSON body is nested too deeply');
        }
        if (!is_array($decoded)) {
            throw new InvalidPay('invalid JSON body');
        }

        return $decoded;
    }
}

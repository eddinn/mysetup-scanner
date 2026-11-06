// How often a report's parts match the MySetup catalogue, with the site's own matcher. Read-only.
//
// For a checkout of the site, in `php artisan tinker --execute="..."`, after defining the reports:
//
//     $reports = ['machine name' => 'the text of its report', ...];
//
// It prints one line per distinct part: machine, part, what the report said, the result and the product.
// Results: identifier (a PCI ID found the product), certain, likely, choose (several candidates) and none.
// Whether a match is the right product is for a person to judge from the output.

$matcher = app(App\Hardware\PastedList\ListMatcher::class);
$lookup = app(App\Hardware\CatalogueLookup::class);
$parser = app(App\Hardware\PastedList\ListParser::class);
$totals = [];

$record = function (string $machine, string $part, string $said, string $result, ?string $product = null) use (&$totals): void {
    $totals[$part][$result] = ($totals[$part][$result] ?? 0) + 1;
    echo implode(' | ', [$machine, $part, $said, $result, $product ?? '-']).PHP_EOL;
};

$productName = fn ($product): string => $product->manufacturer->name.' '.$product->name;

$byList = function (string $machine, string $part, string $label, string $said) use ($matcher, $parser, $record, $productName): void {
    $lines = $parser->parse($label.': '.$said);
    if ($lines === []) {
        $record($machine, $part, $said, 'none');

        return;
    }
    $match = $matcher->match($lines[0]);
    $product = $match->product;
    if ($product === null && $match->candidates !== []) {
        $product = $match->candidates[0];
    }
    $record($machine, $part, $said, $match->state, $product === null ? null : $productName($product));
};

foreach ($reports as $machine => $text) {
    $fields = [];
    foreach (preg_split('/\R/', $text) as $line) {
        if (str_contains($line, '=')) {
            [$name, $value] = explode('=', $line, 2);
            $fields[$name] = $value;
        }
    }

    if (isset($fields['cpu.model'])) {
        $byList($machine, 'cpu', 'CPU', $fields['cpu.model']);
    }

    for ($n = 0; isset($fields["gpu.$n.pci"]) || isset($fields["gpu.$n.model"]); $n++) {
        $said = trim(($fields["gpu.$n.pci"] ?? '').' '.($fields["gpu.$n.subsystem"] ?? '').' '.($fields["gpu.$n.model"] ?? ''));
        $found = null;
        if (isset($fields["gpu.$n.pci"])) {
            [$vendor, $device] = explode(':', $fields["gpu.$n.pci"]);
            [$subVendor, $subDevice] = array_pad(explode(':', $fields["gpu.$n.subsystem"] ?? ''), 2, null);
            $found = $lookup->byIdentifier(App\Enums\HardwareIdentifierType::from('pci'), $vendor, $device, $subVendor ?: null, $subDevice ?: null);
        }
        if ($found !== null) {
            $record($machine, 'gpu', $said, 'identifier', $productName($found->product));
        } elseif (isset($fields["gpu.$n.model"])) {
            $byList($machine, 'gpu', 'GPU', $fields["gpu.$n.model"]);
        } else {
            $record($machine, 'gpu', $said, 'none');
        }
    }

    if (isset($fields['motherboard.model'])) {
        $byList($machine, 'motherboard', 'Motherboard', trim(($fields['motherboard.vendor'] ?? '').' '.$fields['motherboard.model']));
    }

    $seen = [];
    for ($n = 0; isset($fields["memory.$n.size_mb"]); $n++) {
        $part = $fields["memory.$n.part"] ?? '';
        if ($part !== '' && ! isset($seen[$part])) {
            $seen[$part] = true;
            $byList($machine, 'memory', 'Memory', $part);
        }
    }

    $seen = [];
    for ($n = 0; isset($fields["storage.$n.size_bytes"]) || isset($fields["storage.$n.model"]); $n++) {
        $model = $fields["storage.$n.model"] ?? '';
        if ($model !== '' && ! isset($seen[$model])) {
            $seen[$model] = true;
            $byList($machine, 'storage', 'Storage', $model);
        }
    }
}

echo PHP_EOL;
foreach ($totals as $part => $results) {
    echo $part.': '.json_encode($results).PHP_EOL;
}

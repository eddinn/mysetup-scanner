// How often reports' parts match the MySetup catalogue, through the site's own list parser and matcher. Read-only.
//
// For a checkout of the site, in `php artisan tinker --execute="..."`, after defining the reports:
//
//     $reports = ['machine name' => 'the text of its report', ...];
//
// It prints one line per distinct part: machine, part, what the report said, the result and the product, which for "choose"
// is the first candidate. Results: certain, likely, choose (several candidates), none, and "not ticked" for what the review
// offers but does not tick, such as graphics built into the processor. Whether a product is the right one is for a person to
// judge from the output.

$parser = app(App\Hardware\PastedList\ListParser::class);
$matcher = app(App\Hardware\PastedList\ListMatcher::class);
$totals = [];

foreach ($reports as $machine => $text) {
    foreach ($parser->parse($text) as $line) {
        $match = $matcher->match($line);
        $product = $match->product ?? ($match->candidates[0] ?? null);
        $result = $line->include ? $match->state : 'not ticked';
        $totals[$line->role][$result] = ($totals[$line->role][$result] ?? 0) + 1;
        echo implode(' | ', [$machine, $line->role, $line->raw, $line->name, $result,
            $product === null ? '-' : $product->manufacturer->name.' '.$product->name, $match->explanation]).PHP_EOL;
    }
}

echo PHP_EOL;
foreach ($totals as $part => $results) {
    echo $part.': '.json_encode($results).PHP_EOL;
}

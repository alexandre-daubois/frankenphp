<?php
function inner($arg) {
    $local = $arg * 2;
    echo "ok";
}
$intVar = 42;
$floatVar = 3.14;
$stringVar = "hello";
$boolVar = true;
$nullVar = null;
$arr = [1, 2, 3];
$assoc = ['key' => 'value', 'num' => 99];
$nested = ['a' => [1, 2], 'b' => ['x' => true]];
$fh = fopen('php://memory', 'r+');
inner(21);
fclose($fh);

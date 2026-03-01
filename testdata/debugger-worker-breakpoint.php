<?php
$counter = 0;
while (frankenphp_handle_request(function () use (&$counter) {
    $counter++;
    $value = $counter * 10;
    echo "count=$counter";
})) {}

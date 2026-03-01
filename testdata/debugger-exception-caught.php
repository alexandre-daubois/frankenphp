<?php
function throwIt() {
    throw new RuntimeException("caught error");
}
try {
    throwIt();
} catch (RuntimeException $e) {
    $result = $e->getMessage();
}
echo $result;

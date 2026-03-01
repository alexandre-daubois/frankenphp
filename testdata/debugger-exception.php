<?php
function throwUncaught() {
    throw new RuntimeException("something went wrong");
}
throwUncaught();

#!/bin/bash

tail -f ../log/system.log  | grep "commit block \[\|ERROR"


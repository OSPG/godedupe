#!/bin/zsh

mkdir -p tests
cd tests || exit

echo "Testing with big files"
(
mkdir tests_big/
cd tests_big
dd if=/dev/zero of=a1 count=1 bs=2048MB conv=excl
dd if=/dev/zero of=a2 count=1 bs=2048MB conv=excl
dd if=/dev/zero of=a3 count=1 bs=2048MB conv=excl
dd if=/dev/zero of=a4 count=1 bs=2048MB conv=excl
echo a >> a3
echo b >> a4
) &> /dev/null


echo "Running fdupes 3 times"
for i in {1..3}; do; time fdupes -q -r -A -n tests_big &> /dev/null; done

echo "Running godedupe 3 times"
for i in {1..3}; do; time ../godedupe -q -t tests_big -m &>/dev/null; done

echo -e "\n"

echo "Testing with lots of small files"
(
mkdir tests_small
cd tests_small
for i in {1..10000}; do
	echo a > $i
done
mkdir identical
cd identical
for i in {1..10000}; do
	echo $i > $i
done
) &> /dev/null

echo "Running fdupes 3 times"
for i in {1..3}; do; time fdupes -q -r -A -n tests_small &> /dev/null; done

echo "Running godedupe 3 times"
for i in {1..3}; do; time ../godedupe -q -t tests_small -m &>/dev/null; done;


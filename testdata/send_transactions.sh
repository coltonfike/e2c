
python3 main.py '0xe2ddab5e77df6d62f8661650e46d695be1963bf7' 'client' 10000 10000 $1 &
python3 main.py '0xd18aefd325d127fe3e1d6272180a8629413ddc6b' 'password' 10000 10000 $1 &
python3 main.py '0xcf7d7b22af30aadce47930cd234ed34c4488da5e' 'password' 10000 10000 $1 &
python3 main.py '0x82aa48615b89237a0195441da44a63dcbf199f21' 'password' 10000 10000 $1 &
python3 main.py '0x12c825237c38cfe2f879fcd475cb438ed0778d8e' 'password' 10000 10000 $1 &
python3 main.py '0xdee5bc6e1c404c693c0fcf145dcfcb64330eb8bd' 'password' 10000 10000 $1 &
python3 main.py '0xec317a80394abb23c8940b2b7f2d66e0e3c97677' 'password' 10000 10000 $1 &
python3 main.py '0xb48bd20a8c8e687511e36df039c17b8704c2c115' 'password' 10000 10000 $1 &

python3 observe.py $1 > results.txt &
sleep 30
pkill 'python3 main.py'

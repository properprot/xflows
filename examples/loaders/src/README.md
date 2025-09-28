# Compiling xflows.c
In order to compile, I have tested this on Ubuntu 24. Your mileage may vary with other versions of Ubuntu or other operating systems.

```
apt update

apt install clang libbpf-dev -y

clang -O2 -g -target bpf -c xflows.c -o xflows.o
```

Depending on your machine, you may see an error that looks like:
`/usr/include/linux/types.h:5:10: fatal error: 'asm/types.h' file not found`

If you see this, simply run:
`ln -s /usr/include/x86_64-linux-gnu/asm /usr/include/asm`

Once you've ran this, you can recompile and all should be working.
# Naive recursive Fibonacci at 30 — the call-recursion shape.
def fib(n)
  if n < 2
    n
  else
    fib(n - 1) + fib(n - 2)
  end
end
puts fib(30)

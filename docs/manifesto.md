# Why Zhex exists.

Zhex is an experiment in building programs from small functions, called words, that are easy to understand and combine. We want to learn how far clear words, explicit data handling, and composition can take us: whether they make useful programs easier to read, change, and improve.

## Data you can follow

You should be able to read a program and follow its values: where they come from, what each word does with them, and where the results go next. Named values and explicit parameter and return types help make those connections visible. It should also be clear when data is shared or changed, when resources are released, and what happens if an operation fails. The aim is to understand a piece of code without having to hold the whole program in your head.

## Small words that fit together

Forth's idea of words is at the heart of Zhex. Give a useful operation a name, then combine it with other words to express a larger task. A word's signature tells you the types it accepts and returns; its name and body explain what it does. Calls and flows connect words, and the checker catches incompatible types where they meet.

Once you understand a word, you can use that understanding wherever it appears. A well-chosen sequence of words should make the larger task easier to follow. We expect this to give us useful pieces to reuse and test, help us narrow down failures, and let improvements to a shared word benefit the programs that use it. As programs grow, modules should help organize these words. Their design should follow what we learn from writing real programs.

## Useful to people and their tools

AI assistants are part of how people write software, and Zhex should work well with them. Clear words and explicit data flow should help an assistant make a change and help a person review it. Composition gives both a way to work with familiar operations and focus on the pieces involved.

Diagnostic codes, JSON error reports, and optional execution traces give tools information they can work with. These tools should help us understand the program and keep the person responsible for it able to follow the code.

## Errors that help you keep going

An error message should explain what happened and help you decide what to try next. Elm is an inspiration here. Zhex reports source locations and, for some errors, underlines the relevant expression, explains where a value came from, or suggests a correction. Good feedback should let you get back to working on the program with a better understanding of it.

## Performance we can understand

Zhex should be fast enough for the things people want to build with it. That takes work on the implementation, libraries, and data representations. Clear data handling should help us understand where copying, allocation, and mutation happen. Small words give us focused operations to investigate and improve, with the benefits reaching the programs that compose them.

We use benchmarks to measure execution time and allocations, recording the workload, machine, and method so results can be checked. Execution traces help explain what a program did. Finding where it spends time or allocates memory requires measurements of those costs. We want those measurements to guide our decisions.

## An experiment worth learning from

The main goal is learning. We will judge these ideas by writing and using real programs. If the benefits we expect do not show up, we will abandon the effort. Understanding what worked, what failed, and why will still make the experiment worthwhile.

Enjoying the work matters too. There is pleasure in finding a good name, making a few words fit together, and seeing a program work. Zhex should be comfortable to write by hand and easy to review when an assistant helps. How it feels to work in the language is part of what we want to learn.

## Where it comes from

Zhex starts with a lasting affection for Forth. I have always liked its small words and the way they compose, even when I struggled to find a place for it in my own work. Elm and Gleam are influences for their functional approach and care for the programmer's experience. Ruby's emphasis on enjoying programming matters here too. Python is a reference for indentation and readable syntax, and Lisp for building programs from small, composable pieces.

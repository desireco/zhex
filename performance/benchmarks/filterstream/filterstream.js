// Print stdin lines containing "ERROR", streaming until end of input. Node and Bun.
const readline = require("readline");
const rl = readline.createInterface({ input: process.stdin });
rl.on("line", (line) => {
    if (line.includes("ERROR")) {
        console.log(line);
    }
});

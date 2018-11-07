module.exports.helloWorld = (req, res) => {
    console.log("hello world")
    res.send(req.method)
}

module.exports.hello = (req, res) => {
    console.log("logging inside container - hello")
    res.send("sending through response - hello")
}

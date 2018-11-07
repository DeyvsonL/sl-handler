const functions = require('./code.js')

let [name, query, method, headers] = process.argv.slice(2)

let request = {
    protocol: 'http',
    name: name,
    path: `/${name}`,
    query: JSON.parse(query),
    method: method.toUpperCase(),
    headers: JSON.parse(headers),
    body: {
        raw: process.stdin,
        text: async function () {
            return await new Promise(resolve => this.raw.on('data', data => resolve(data)))
        },
        json: async function () {
            return JSON.parse(await this.text())
        }
    }
}

let response = {
    protocol: 'http',
    headers: {},
    _body: '',
    code: 200,
    finished: false,
    set: function (header, value) {
        if (this.finished) throw 'response already finished'

        if (header instanceof String) {
            this._headers = { [header.toLowerCase()]: value.toLowerCase() }
        } else {
            this._headers = header
        }
    },
    send: function (obj) {
        if (this.finished) throw 'response already finished'

        this._body = JSON.stringify(obj)
        this.set({ 'content-type': 'application/json', 'content-length': this._body.length })
        this.finished = true
    },
    write: function (text) {
        if (this.finished) throw 'response already finished'

        if (!text instanceof String) {
            throw 'text should be string, use send(obj) for objects'
        }
        this._body = text
        this.set({ 'content-type': 'plain/text', 'content-length': this._body.length })
        this.finished = true
    }
}

let matchingFunctions = Object.keys(functions).filter(name => name == request.name)
if (matchingFunctions.length == 0) {
    response.code = 404
    response.write('Function not found')
} else {
    functions[matchingFunctions[0]](request, response)
}

process.stdout.write(JSON.stringify({ code: response.code, headers: response._headers, body: response._body }))

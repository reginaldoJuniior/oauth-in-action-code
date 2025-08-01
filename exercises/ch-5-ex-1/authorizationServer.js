var express = require("express");
var url = require("url");
var bodyParser = require('body-parser');
var randomstring = require("randomstring");
var cons = require('consolidate');
var nosql = require('nosql').load('database.nosql');
var querystring = require('querystring');
var __ = require('underscore');
const {decode} = require("qs/lib/utils");
const timers = require("node:timers");
__.string = require('underscore.string');

var app = express();

app.use(bodyParser.json());
app.use(bodyParser.urlencoded({ extended: true })); // support form-encoded bodies (for the token endpoint)

app.engine('html', cons.underscore);
app.set('view engine', 'html');
app.set('views', 'files/authorizationServer');
app.set('json spaces', 4);

// authorization server information
var authServer = {
	authorizationEndpoint: 'http://localhost:9001/authorize',
	tokenEndpoint: 'http://localhost:9001/token'
};

// client information
var clients = [
	{
		"client_id": "oauth-client-1",
		"client_secret": "oauth-client-secret-1",
		"redirect_uris": ["http://localhost:9000/callback"],
	}
];

var codes = {};

var requests = {};

var getClient = function(clientId) {
	return __.find(clients, function(client) { return client.client_id === clientId; });
};

app.get('/', function(req, res) {
	res.render('index', {clients: clients, authServer: authServer});
});

app.get("/authorize", function(req, res){
	let client = getClient(req.query.client_id);

	if (!client) {
		console.log('Unknown client %s', req.query.client_id);
		res.render('error', {error: 'Unknown client'});
	} else if (!__.contains(client.redirect_uris, req.query.redirect_uri)) {
		console.log('Mismatched redirect URI %s for client %', req.query.redirect_uri, req.query.client_id);
		res.render('error', {error: 'Invalid redirect URI'});
	}

	let reqid = randomstring.generate(8);
	requests[reqid] = req.query;

	res.render('approve', { client: client, reqid: reqid });
});

app.post('/approve', function(req, res) {
	let reqid = req.body.reqid;
	let query = requests[reqid];
	delete requests[reqid];
	if (!query) {
		res.render('error', {error: 'No matching authorization request'});
	}

	if (req.body.approve) {
		if (query.response_type === 'code') {
			let code = randomstring.generate(8);
			codes[code] = { request: query };

			let urlParsed = buildUrl(query.redirect_uri, {code: code, state: query.state});
			res.redirect(urlParsed)
		} else {
			let urlParsed = buildUrl(query.redirect_uri, {error: 'unsupported_response_type'});
			res.redirect(urlParsed);
		}
	} else {
		let urlPassed = buildUrl(query.redirect_uri, {error: 'access_denied'});
		res.redirect(urlPassed);
	}
});

app.post("/token", function(req, res){
	let auth = req.headers['authorization'];
	if (auth) {
		let clientCredentials = decodeClientCredentials(auth);
		let clientId = clientCredentials.id;
		let clientSecret = clientCredentials.secret;

		if (req.body.client_id) {
			if (clientId) {
				res.status(401).json({error: 'invalid_client'});
			}

			clientId = req.body.client_id;
			clientSecret = req.body.client_secret;
		}

		let client = getClient(clientId);
		if (!client) {
			console.log('Unknown client %s', clientId);
			res.status(401).json({error: 'invalid_client'});
		}

		if (client.client_secret !== clientSecret) {
			console.log('Invalid client secret for client %s', clientId);
			res.status(401).json({error: 'invalid_client'});
		}

		if (req.body.grant_type === 'authorization_code') {
			let code = codes[req.body.code];

			if (code) {
				delete codes[req.body.code]; // burn our code, it's been used

				if (code.request.client_id === clientId) {
					let access_token = randomstring.generate()
					nosql.insert({access_token: access_token, client_id: clientId})

					console.log("Including access token %s", access_token);
					let token_response = {
						access_token: access_token,
						token_type: 'Bearer'
					}
					res.status(200).json(token_response);
				} else {
					console.log('Authorization code %s does not match client %s', req.body.code, clientId);
					res.status(400).json({error: 'invalid_grant'});
				}
			} else {
				console.log('Unknown authorization code %s', req.body.code);
				res.status(400).json({error: 'invalid_grant'});
			}
		} else {
			res.status(400).json({error: 'unsupported_grant_type'});
		}
	}


});

var buildUrl = function(base, options, hash) {
	var newUrl = url.parse(base, true);
	delete newUrl.search;
	if (!newUrl.query) {
		newUrl.query = {};
	}
	__.each(options, function(value, key, list) {
		newUrl.query[key] = value;
	});
	if (hash) {
		newUrl.hash = hash;
	}
	
	return url.format(newUrl);
};

var decodeClientCredentials = function(auth) {
	var clientCredentials = Buffer.from(auth.slice('basic '.length), 'base64').toString().split(':');
	var clientId = querystring.unescape(clientCredentials[0]);
	var clientSecret = querystring.unescape(clientCredentials[1]);	
	return { id: clientId, secret: clientSecret };
};

app.use('/', express.static('files/authorizationServer'));

// clear the database
nosql.clear();

var server = app.listen(9001, 'localhost', function () {
  var host = server.address().address;
  var port = server.address().port;

  console.log('OAuth Authorization Server is listening at http://%s:%s', host, port);
});
 

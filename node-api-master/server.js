const express = require('express');
const bodyParser = require('body-parser');
const jwt = require('jsonwebtoken');
const { Pool } = require('pg');
const bcrypt = require('bcrypt');

const app = express();
app.use(bodyParser.json());

const SECRET_KEY = 'your_secret_key';
const SALT_ROUNDS = 10;

const pool = new Pool({
    user: 'postgres',
    host: 'localhost',
    database: 'wallet_engine',
    password: 'admin',
    port: 5433,
});

// Register endpoint
app.post('/register', async (req, res) => {
    const { username, password } = req.body;
    if (!username || !password) {
        return res.status(400).json({ message: 'Username and password are required' });
    }

    try {
        const existingUser = await pool.query('SELECT * FROM users WHERE username = $1', [username]);
        if (existingUser.rows.length > 0) {
            return res.status(400).json({ message: 'User already exists' });
        }

        // Hash the password
        const hashedPassword = await bcrypt.hash(password, SALT_ROUNDS);

        const newUser = await pool.query(
            'INSERT INTO users (username, password) VALUES ($1, $2) RETURNING id',
            [username, hashedPassword]
        );
        const userId = newUser.rows[0].id;

        // Initialize user pocket with balance 10000
        await pool.query('INSERT INTO pockets (user_id, balance) VALUES ($1, $2)', [userId, 10000]);

        res.json({ message: 'User registered successfully' });
    } catch (err) {
        console.error('Registration error:', err);
        res.status(500).json({ message: 'Internal server error' });
    }
});

// Login endpoint
app.post('/login', async (req, res) => {
    const { username, password } = req.body;

    try {
        const user = await pool.query('SELECT * FROM users WHERE username = $1', [username]);
        if (user.rows.length === 0) {
            return res.status(401).json({ message: 'Invalid credentials' });
        }

        // Compare the provided password with the stored hash
        const isValidPassword = await bcrypt.compare(password, user.rows[0].password);
        if (!isValidPassword) {
            return res.status(401).json({ message: 'Invalid credentials' });
        }

        const token = jwt.sign({ username }, SECRET_KEY, { expiresIn: '1h' });
        res.json({ token });
    } catch (err) {
        console.error('Login error:', err);
        res.status(500).json({ message: 'Internal server error' });
    }
});

// Protected endpoint to get pocket balance
app.get('/pocket', async (req, res) => {
    const token = req.headers['authorization'];
    if (!token) return res.status(403).json({ message: 'No token provided' });

    jwt.verify(token, SECRET_KEY, async (err, decoded) => {
        if (err) return res.status(500).json({ message: 'Failed to authenticate token' });

        try {
            const user = await pool.query('SELECT id FROM users WHERE username = $1', [decoded.username]);
            if (user.rows.length === 0) {
                return res.status(404).json({ message: 'User not found' });
            }

            const userId = user.rows[0].id;
            const pocket = await pool.query('SELECT balance FROM pockets WHERE user_id = $1', [userId]);
            res.json({ balance: pocket.rows[0].balance });
        } catch (err) {
            console.error('Pocket balance error:', err);
            res.status(500).json({ message: 'Internal server error' });
        }
    });
});

app.listen(3000, () => console.log('Node.js API running on http://localhost:3000'));
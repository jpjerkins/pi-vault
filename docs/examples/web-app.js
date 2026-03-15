#!/usr/bin/env node
/**
 * Example: Express web app using vault secrets
 */

const express = require('express');
const { execSync } = require('child_process');

/**
 * Get secret from vault
 */
function getSecret(name) {
    try {
        const secret = execSync(`vault-get ${name}`).toString().trim();
        return secret;
    } catch (error) {
        console.error(`Failed to get secret '${name}':`, error.message);
        throw error;
    }
}

async function main() {
    console.log('🚀 Starting web application...');

    // Get secrets from vault
    console.log('Fetching secrets...');
    const dbPassword = getSecret('db_password');
    const apiKey = getSecret('openai_api_key');
    const jwtSecret = getSecret('jwt_secret');

    // Configure database connection
    const dbConfig = {
        host: 'localhost',
        port: 5432,
        database: 'myapp',
        user: 'myapp',
        password: dbPassword
    };

    // Configure Express app
    const app = express();
    app.use(express.json());

    // API routes
    app.get('/api/status', (req, res) => {
        res.json({
            status: 'ok',
            timestamp: new Date().toISOString()
        });
    });

    app.post('/api/generate', async (req, res) => {
        // Use OpenAI API with secret key
        const { prompt } = req.body;

        // Call OpenAI API with apiKey
        // ... implementation ...

        res.json({ result: 'Generated content' });
    });

    // Start server
    const PORT = process.env.PORT || 3000;
    app.listen(PORT, () => {
        console.log(`✓ Server running on port ${PORT}`);
    });
}

main().catch(error => {
    console.error('Fatal error:', error);
    process.exit(1);
});

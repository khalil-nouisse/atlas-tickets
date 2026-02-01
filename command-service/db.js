require('dotenv').config();
const { Pool } = require('pg');

const pool = new Pool({
  user: process.env.DB_USER || 'atlas',
  password: process.env.DB_PASSWORD || 'your_password',
  host: process.env.DB_HOST || 'localhost',
  port: parseInt(process.env.DB_PORT || '5432', 10),
  database: process.env.DB_NAME || 'postgres'
});

// The pool will emit an error on behalf of any idle clients
// it contains if a backend error or network partition happens
pool.on('error', (err, client) => {
  console.error('Unexpected error on idle client', err);
  process.exit(-1);
});

const testConnection = async () => {
  const MAX_RETRIES = 10;
  const RETRY_DELAY = 5000; // 5 seconds

  for (let i = 0; i < MAX_RETRIES; i++) {
    try {
      console.log(`Attempting to connect to database (Attempt ${i + 1}/${MAX_RETRIES})...`);
      const client = await pool.connect();
      console.log('Database connected successfully');
      client.release();
      return; // Success
    } catch (err) {
      console.error(`Database connection failed (Attempt ${i + 1}/${MAX_RETRIES}):`, err.message);
      if (i === MAX_RETRIES - 1) {
        console.error("Max retries reached. Exiting...");
        process.exit(1);
      }
      await new Promise(resolve => setTimeout(resolve, RETRY_DELAY));
    }
  }
};

module.exports = {
  query: (text, params) => pool.query(text, params),
  testConnection
};
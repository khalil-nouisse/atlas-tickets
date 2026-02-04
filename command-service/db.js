require('dotenv').config();
const { Pool } = require('pg');

const pool = new Pool({
  connectionString: process.env.POSTGRES_URL,
});

pool.on('error', (err) => {
  console.error('Unexpected error on idle client', err);
  process.exit(1);
});

const testConnection = async () => {
  const MAX_RETRIES = 10;
  const RETRY_DELAY = 5000;

  for (let i = 0; i < MAX_RETRIES; i++) {
    try {
      console.log(`Attempting to connect to database (Attempt ${i + 1}/${MAX_RETRIES})...`);
      const client = await pool.connect();
      console.log('Database connected successfully');
      client.release();
      return;
    } catch (err) {
      console.error(`Database connection failed (Attempt ${i + 1}/${MAX_RETRIES}):`, err.message);

      if (i === MAX_RETRIES - 1) {
        console.error('Max retries reached. Exiting...');
        process.exit(1);
      }

      await new Promise(resolve => setTimeout(resolve, RETRY_DELAY));
    }
  }
};

module.exports = {
  query: (text, params) => pool.query(text, params),
  testConnection,
};
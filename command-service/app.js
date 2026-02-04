const express = require('express');
const db = require('./db');
const app = express();
const fs = require('fs');
const path = require('path');
const api = require('./routes/ticket');
const { connectRabbitMQ } = require('./services/rabbitMqService');
const swaggerUi = require('swagger-ui-express');
const swaggerSpecs = require('./swagger');

// Define Initialization Function 
async function initializeDatabase() {
  try {
    const scriptsDir = path.join(__dirname, '..', 'docker', 'postgres', 'scripts');
    const scripts = ['01-schema.sql', '02-seed-data.sql', '03-indexes.sql'];

    console.log('Starting automated database initialization...');

    for (const script of scripts) {
      const sqlPath = path.join(scriptsDir, script);
      if (fs.existsSync(sqlPath)) {
        const sql = fs.readFileSync(sqlPath, 'utf8');
        console.log(`Running Script: ${script}`);
        await db.query(sql);
      } else {
        console.warn(`Script not found: ${script}`);
      }
    }
    console.log('Database initialization completed successfully.');
  } catch (err) {
    console.error('Error during database initialization:', err);
    throw err;
  }
}

// Middleware & Routes
app.use(express.urlencoded({ extended: false }));
app.use(express.json());

// Health check endpoint (liveness)
app.get('/health', (req, res) => {
  // Simple check - is the process running?
  res.status(200).json({ status: 'ok' });
});

// Readiness check
app.get('/ready', async (req, res) => {
  try {
    // Check RabbitMQ connection
    if (!rabbitmqChannel || !rabbitmqChannel.connection) {
      return res.status(503).json({ 
        status: 'not ready', 
        reason: 'RabbitMQ not connected' 
      });
    }
    
    res.status(200).json({ 
      status: 'ready',
      connections: {
        rabbitmq: 'connected'
      }
    });
  } catch (error) {
    res.status(503).json({ 
      status: 'not ready', 
      error: error.message 
    });
  }
});

app.use('/api-docs', swaggerUi.serve, swaggerUi.setup(swaggerSpecs));
app.use('/api/tickets', api);

// Startup Chain
db.testConnection()
  .then(async () => {
    // Initialize DB Tables (Schema + Seeds)
    await initializeDatabase();
  })
  .then(() => {
    // Connect to Message Broker
    return connectRabbitMQ();
  })
  .then(() => {
    // Start HTTP Server
    app.listen(5000, () => {
      console.log(" CONNECTED TO RABBITMQ & SERVER RUNNING ON PORT 5000");
    });
  })
  .catch((err) => {
    console.error("Critical Startup Error:", err);
    process.exit(1);
  });

module.exports = app;
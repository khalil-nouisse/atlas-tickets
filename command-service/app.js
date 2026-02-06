const express = require('express');

const app = express();
const fs = require('fs');
const path = require('path');
const api = require('./routes/ticket');
const { connectRabbitMQ, isConnected } = require('./services/rabbitMqService');
const swaggerUi = require('swagger-ui-express');
const swaggerSpecs = require('./swagger');



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
    if (!isConnected()) {
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
connectRabbitMQ()
  .then(() => {
    // Start HTTP Server
    const PORT = process.env.PORT || 3000;
    app.listen(PORT, () => {
      console.log(` CONNECTED TO RABBITMQ & SERVER RUNNING ON PORT ${PORT}`);
    });
  })
  .catch((err) => {
    console.error("Critical Startup Error:", err);
    process.exit(1);
  });

module.exports = app;
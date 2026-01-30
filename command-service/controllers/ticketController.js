const { publishToQueue } = require('../services/rabbitMqService');
const { v4: uuidv4 } = require('uuid');

/**
 * @swagger
 * /api/tickets:
 *   post:
 *     summary: Buy a ticket
 *     tags: [Tickets]
 *     requestBody:
 *       required: true
 *       content:
 *         application/json:
 *           schema:
 *             type: object
 *             required:
 *               - match_id
 *               - category
 *               - quantity
 *               - user_id
 *             properties:
 *               match_id:
 *                 type: string
 *               category:
 *                 type: string
 *               quantity:
 *                 type: integer
 *               user_id:
 *                 type: string
 *     responses:
 *       202:
 *         description: Request received
 *       400:
 *         description: Invalid input
 *       500:
 *         description: Internal Server Error
 */
const buyTicket = async (req, res) => {
    try {
        const { match_id, category, quantity, user_id } = req.body;

        // Basic validation
        if (!match_id || !category || !quantity || !user_id) {
            return res.status(400).json({ error: "match_id, category, quantity, and user_id are required" });
        }

        // Anti-scalping rule
        if (quantity < 1 || quantity > 4) {
            return res.status(400).json({ error: "Quantity must be between 1 and 4" });
        }

        const payload = {
            event: "TICKET_REQUESTED",
            request_id: uuidv4(),
            data: {
                match_id,
                category,
                quantity,
                user_id,
                timestamp: new Date().toISOString()
            }
        };

        // Publish Event
        await publishToQueue('ticket_queue', payload);

        res.status(202).json({ message: "Request received. Processing..." });
    } catch (err) {
        console.error(err);
        res.status(500).json({ error: "Internal Server Error" });
    }
}

module.exports = { buyTicket }
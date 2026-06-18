// file: server/routes/users.js

/**
 * @tag-block api-endpoint
 * @tag route: "/v1/users/:id/promote"
 * @tag method: "POST"
 * @tag layer: "controller"
 * @tag security: "rbac"
 * @tag roles: ["admin", "super-admin"]
 */
async function promoteUser(req, res) {
    const { id } = req.params;
    
    // @tag-inline audit: "Mutates critical user role data"
    await db.users.update(id, { role: 'admin' });
    
    res.status(200).send({ success: true });
}

/**
 * @tag-block api-endpoint
 * @tag route: "/v1/users"
 * @tag method: "GET"
 * @tag layer: "controller"
 * @tag security: "public"
 */
async function listUsers(req, res) {
    const users = await db.users.findAll();
    
    // @tag-inline pii: "Returns user email addresses"
    res.status(200).json(users);
}

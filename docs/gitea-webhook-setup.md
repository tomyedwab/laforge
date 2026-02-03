# Gitea Webhook Configuration

This document describes how to configure Gitea to send webhook events to the orchestrator service.

## Prerequisites

1. The orchestrator service must be running (via `docker-compose up orchestrator`)
2. You need admin access to the Gitea repository
3. Gitea must be configured to allow webhook requests to the orchestrator

## Configuration Steps

### 1. Configure Gitea's Allowed Host List

Gitea restricts which hosts webhooks can call for security. You need to add the orchestrator to the allowed list.

**Option A: If running orchestrator on the same Docker network as Gitea**

Add to Gitea's `app.ini` configuration file:

```ini
[webhook]
ALLOWED_HOST_LIST = private
```

This allows webhooks to call services on private Docker networks.

**Option B: If running orchestrator on the host or a different Docker network**

If you're using `host.docker.internal` or need to specify the exact host, add:

```ini
[webhook]
ALLOWED_HOST_LIST = private, host.docker.internal
```

Or specify the exact IP/hostname:

```ini
[webhook]
ALLOWED_HOST_LIST = private, 192.168.65.254
```

After updating `app.ini`, restart Gitea:

```bash
docker-compose restart gitea
```

### 2. Generate a Webhook Secret (Optional but Recommended)

Generate a secure random string to use as the webhook secret:

```bash
openssl rand -hex 32
```

Save this secret - you'll need it in step 4.

### 3. Set the Webhook Secret Environment Variable

Create a `.env` file in the root of the repository (or add to your existing one):

```bash
WEBHOOK_SECRET=your-generated-secret-here
```

Then restart the orchestrator service:

```bash
docker-compose restart orchestrator
```

### 4. Configure the Webhook in Gitea

1. Navigate to your repository in Gitea (e.g., `http://localhost:3010/tom/laforge`)
2. Click on **Settings** → **Webhooks**
3. Click **Add Webhook** → **Gitea**
4. Configure the webhook with the following settings:

   - **Target URL**:
     - If orchestrator is on the same Docker network: `http://orchestrator:8080/webhook`
     - If orchestrator is on host or different network: `http://host.docker.internal:8080/webhook`
   - **HTTP Method**: `POST`
   - **POST Content Type**: `application/json`
   - **Secret**: Enter the secret you generated in step 2 (if you're using webhook authentication)
   - **Trigger On**: Select **Custom Events** and check:
     - Pull Request
     - Pull Request Review
     - Pull Request Review Comment
     - Issue Comment
   - **Branch filter**: Leave empty (all branches)
   - **Active**: Check this box

5. Click **Add Webhook**

### 5. Test the Webhook

1. Click on the webhook you just created
2. Scroll down to **Recent Deliveries**
3. Click **Test Delivery**
4. Check the orchestrator logs to verify the webhook was received:

```bash
docker-compose logs orchestrator
```

You should see a log entry like:

```json
{
  "time": "2026-02-02T22:00:00Z",
  "level": "INFO",
  "msg": "webhook received",
  "event": "pull_request",
  "action": "opened",
  "repository": "tom/laforge",
  "number": 1,
  "sender": "tom"
}
```

## Webhook Events

The orchestrator is configured to handle the following webhook events, matching the triggers from `.gitea/workflows/agent.yaml`:

### Pull Request Events
- **Trigger**: Pull request created, reopened, or assigned
- **Event Type**: `pull_request`
- **Actions**: `opened`, `reopened`, `assigned`

### Pull Request Review Events
- **Trigger**: Review submitted or edited
- **Event Type**: `pull_request_review`
- **Actions**: `submitted`, `edited`

### Pull Request Review Comment Events
- **Trigger**: Comment on a PR diff created or edited
- **Event Type**: `pull_request_review_comment`
- **Actions**: `created`, `edited`

### Issue Comment Events
- **Trigger**: Comment on a PR created or edited
- **Event Type**: `issue_comment`
- **Actions**: `created`, `edited`
- **Note**: The orchestrator should filter to only handle comments on PRs

## Troubleshooting

### Webhook delivery fails with "webhook can only call allowed HTTP servers"

This error means Gitea's `ALLOWED_HOST_LIST` is blocking the webhook. See Step 1 above to configure the allowed host list properly.

Example error:
```
webhook can only call allowed HTTP servers (check your webhook.ALLOWED_HOST_LIST setting), deny 'host.docker.internal(192.168.65.254:8080)'
```

**Solution**: Add the appropriate host to Gitea's `app.ini`:
```ini
[webhook]
ALLOWED_HOST_LIST = private, host.docker.internal
```

### Other webhook delivery failures
- Verify the orchestrator service is running: `docker-compose ps orchestrator`
- Check orchestrator logs: `docker-compose logs orchestrator`
- Verify the webhook URL uses the correct hostname (service name or host.docker.internal)
- Ensure both services are on the same Docker network (`gitea`) if using service names

### Signature validation fails
- Verify the `WEBHOOK_SECRET` environment variable matches the secret configured in Gitea
- Check orchestrator logs for "webhook signature validation failed" messages
- Ensure you've restarted the orchestrator after setting the environment variable

### No logs appear
- Check if the webhook is set to **Active**
- Verify the correct events are selected in the webhook configuration
- Check Gitea's webhook delivery history for error messages

## Next Steps

Currently, the orchestrator only logs webhook events. Future PRs will add:
- Event processing logic
- LLM integration for automated responses
- Configuration system for orchestrator behavior

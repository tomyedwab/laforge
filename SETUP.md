# Laforge Agent Configuration

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

Save this secret - you'll need it in step 5.

### 3. Generate a Gitea API Token

The orchestrator needs a Gitea API token to update commit statuses (to show when jobs are queued, running, or complete).

1. Log into Gitea (e.g., `http://localhost:3010`)
2. Navigate to **Settings** → **Applications**
3. Under **Generate New Token**, enter:
   - **Token Name**: `orchestrator` (or any descriptive name)
   - **Select permissions**:
     - **Required**: Check **write:repository** (allows creating commit statuses)
     - Or simply check the top-level **repository** checkbox (includes all repository permissions)
4. Click **Generate Token**
5. **Important**: Copy the token immediately - you won't be able to see it again

**Note**: The token must be created by a user who has write access to the repository. If you're testing on a personal repository, use your own account to generate the token.

### 4. Configure Laforge

Copy the example configuration file and customize it:

```bash
cp laforge-config.example.yaml laforge-config.yaml
```

Edit `laforge-config.yaml` with your settings:

```yaml
server:
  port: "8080"
  webhook_secret: "your-generated-secret-here"  # From step 2

redis:
  address: "redis:6379"

gitea:
  url: "http://gitea:3000"
  external_url: "http://localhost:3010"  # URL users access Gitea at
  token: "your-gitea-token-here"  # From step 3

worker:
  concurrency: 5

bot:
  username: "laforge"
  email: "laforge@example.com"

# Model configuration - see laforge-config.example.yaml for full options
models:
  sonnet:
    model_id: "claude-sonnet-4-5-20250929"
    image: "laforge/claudecode"

prompts:
  default_type: "implement"
  default_model: "sonnet"
```

Start the orchestrator service:

```bash
docker-compose up -d orchestrator
```

To restart after configuration changes:

```bash
docker-compose restart orchestrator
```

### 5. Configure the Webhook in Gitea

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

### 6. Test the Webhook

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
- **Does NOT trigger on**: `synchronized` (commits), `closed`, `edited`, `unassigned`

### Pull Request Review Events
- **Trigger**: Review submitted (all types: approved, changes_requested, commented)
- **Event Type**: `pull_request_review`
- **Actions**: `submitted`

### Pull Request Review Comment Events
- **Trigger**: Comment on a PR diff created or edited
- **Event Type**: `pull_request_review_comment`
- **Actions**: `created`, `edited`

### Issue Comment Events
- **Trigger**: Comment on a PR created or edited
- **Event Type**: `issue_comment`
- **Actions**: `created`, `edited`
- **Note**: Only comments on pull requests trigger the agent, not regular issue comments

### Event Filtering

The orchestrator implements smart event filtering to prevent unnecessary agent runs:

1. **Bot Self-Filtering**: Events triggered by the bot user (configured via `BOT_USERNAME`) are automatically ignored to prevent infinite loops. When the bot comments on a PR, it won't trigger itself.

2. **Action Whitelisting**: Only specific actions within each event type trigger the agent. For example:
   - ✅ `pull_request.opened` - Triggers agent
   - ✅ `pull_request.assigned` - Triggers agent
   - ❌ `pull_request.synchronized` - Does NOT trigger (commits to PR)
   - ❌ `pull_request.closed` - Does NOT trigger

3. **PR vs Issue Detection**: For `issue_comment` events, the orchestrator verifies the comment is on a pull request, not a regular issue, before triggering the agent.

4. **Multiple Events**: Multiple events for the same PR can be queued and will be processed in order. The Redis lock ensures only one agent runs per PR at a time, even if multiple events are queued.

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

### Commit status update fails with "not found"

If you see this error in the orchestrator logs:
```json
{
  "level": "ERROR",
  "msg": "failed to update status to pending",
  "error": "failed to create status: not found"
}
```

This typically means the Gitea API token doesn't have the correct permissions. To fix:

1. **Check token permissions**: The token must have **write:repository** or **repo:status** scope
   - In Gitea, go to **Settings** → **Applications**
   - Delete the old token
   - Create a new token with **repository** permissions (includes status updates)
   - Update the `GITEA_TOKEN` environment variable
   - Restart the orchestrator: `docker-compose restart orchestrator`

2. **Verify the repository exists**: Ensure the repository name in the webhook payload matches exactly
   - Check orchestrator logs for the repository name being used
   - Repository format should be `owner/repo`

3. **Check if commit exists**: For fork-based PRs, ensure the commit is accessible to the base repository
   - The commit must be visible to Gitea at the time the webhook fires
   - This is usually automatic when a PR is created

4. **API token user permissions**: Ensure the user who owns the API token has write access to the repository
   - The token owner must be able to push to the repository or have collaborator access

If the issue persists, check the orchestrator logs for more detailed error information including HTTP status codes.

## Claude Code

To generate a Claude Code OAuth token tied to your Pro/Max account, run this command locally:

```bash
claude setup-token
```

# TeamCity Label Feature Test

## Usage Examples

1. **Single label:**
   ```bash
   ./tctest pr 123 --label=urgent
   ```

2. **Multiple labels:**
   ```bash
   ./tctest pr 123 --label=urgent,regression
   ```

3. **Multiple labels (alternative syntax):**
   ```bash
   ./tctest pr 123 --label=urgent --label=regression
   ```

4. **With branch command:**
   ```bash
   ./tctest branch main "TestAcc" --label=nightly,main-branch
   ```

5. **Via environment (if needed):**
   ```bash
   # Note: Currently no environment variable is configured for labels
   # but could be added if needed
   ```

## How it works

1. The PR command triggers the build as usual
2. After the build is queued, it calls the TeamCity REST API to add labels
3. Each label is sent as a separate POST request to `/app/rest/2018.1/builds/id:{buildID}/tags`
4. The request body contains just the label text with `Content-Type: text/plain`

## API Details

- Endpoint: `POST /app/rest/2018.1/builds/id:{buildID}/tags`
- Content-Type: `text/plain`
- Body: The label text (one label per request)

## Error Handling

- If labeling fails, it shows a warning but doesn't fail the entire operation
- Empty labels are skipped
- Each label is processed individually, so one failure doesn't affect others

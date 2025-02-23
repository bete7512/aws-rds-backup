package notification

const (
	successEmailTemplate = `
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif;">
    <h2>RDS Backup Successful</h2>
    <p>The RDS backup has been completed successfully.</p>
    <ul>
        <li><strong>Database:</strong> {{.DBIdentifier}}</li>
        <li><strong>Snapshot ID:</strong> {{.SnapshotID}}</li>
        <li><strong>Backup Time:</strong> {{.BackupTime}}</li>
        <li><strong>S3 Location:</strong> {{.S3Location}}</li>
    </ul>
    <p>This is an automated message. Please do not reply.</p>
</body>
</html>`

	failureEmailTemplate = `
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif;">
    <h2 style="color: #ff0000;">RDS Backup Failed</h2>
    <p>The RDS backup operation has encountered an error.</p>
    <ul>
        <li><strong>Database:</strong> {{.DBIdentifier}}</li>
        <li><strong>Attempted Snapshot ID:</strong> {{.SnapshotID}}</li>
        <li><strong>Error Time:</strong> {{.BackupTime}}</li>
        <li><strong>Error Message:</strong> {{.ErrorMessage}}</li>
    </ul>
    <p>Please check the AWS console and logs for more details.</p>
    <p>This is an automated message. Please do not reply.</p>
</body>
</html>`
)
#!/usr/bin/env python3
"""
Example: Database backup script using vault secrets
"""

import subprocess
import os
from datetime import datetime

def get_secret(name):
    """Get secret from vault"""
    result = subprocess.run(
        ['vault-get', name],
        capture_output=True,
        text=True,
        check=True
    )
    return result.stdout.strip()

def main():
    print("📦 Starting database backup...")

    # Get secrets from vault
    print("Fetching credentials...")
    db_password = get_secret('db_password')
    s3_access_key = get_secret('aws_s3_access_key')
    s3_secret_key = get_secret('aws_s3_secret_key')

    # Set environment variables
    os.environ['PGPASSWORD'] = db_password
    os.environ['AWS_ACCESS_KEY_ID'] = s3_access_key
    os.environ['AWS_SECRET_ACCESS_KEY'] = s3_secret_key

    # Backup database
    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    backup_file = f'backup_{timestamp}.sql'

    print(f"Dumping database to {backup_file}...")
    subprocess.run([
        'pg_dump',
        '-h', 'localhost',
        '-U', 'myuser',
        '-d', 'mydb',
        '-f', backup_file
    ], check=True)

    # Upload to S3
    print("Uploading to S3...")
    subprocess.run([
        'aws', 's3', 'cp',
        backup_file,
        f's3://my-backups/db/{backup_file}'
    ], check=True)

    # Clean up local file
    os.remove(backup_file)

    print("✓ Backup complete!")

if __name__ == '__main__':
    main()

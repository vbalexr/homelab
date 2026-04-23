#!/usr/bin/env python3
"""
Generate Mosquitto passwords in proper argon2id format ($7$) for version 2.1.2+

Mosquitto 2.1.2 requires argon2id hashed passwords ($7$ format).
This script reads a plain-text password file and outputs the hashed version.

USAGE:
1. Create a plain-text password file with format: username:password
   Example:
     hassio:Adm1n_9@55w0rD
     z2m:Z2m_9@55w0rD
     device:D3v1ce_9@55w0rD

2. Install dependency: pip3 install argon2-cffi

3. Run: python3 generate-passwords.py <password-file>

4. Script outputs base64-encoded hashed passwords for Kubernetes Secret

5. Update your Secret resource in your overlay with the output

6. Redeploy: kubectl apply -k <your-overlay>

IMPORTANT: Never commit plain-text password files to version control!
Use .gitignore: echo "*.passwd" >> .gitignore
"""
import base64
import sys
import os
from argon2 import PasswordHasher
from argon2.exceptions import VerifyMismatchError

def generate_from_file(plaintext_file):
    """Read plain-text password file and hash with argon2id"""
    
    # Verify input file exists
    if not os.path.exists(plaintext_file):
        print(f"ERROR: File not found: {plaintext_file}", file=sys.stderr)
        sys.exit(1)
    
    # Read the plain-text passwords
    try:
        with open(plaintext_file, 'r') as f:
            lines = f.readlines()
    except Exception as e:
        print(f"ERROR: Could not read file: {e}", file=sys.stderr)
        sys.exit(1)
    
    # Initialize argon2 hasher with Mosquitto-compatible parameters
    # time_cost=2, memory_cost=65536 (65MB), parallelism=1
    hasher = PasswordHasher(
        time_cost=2,
        memory_cost=65536,
        parallelism=1,
        hash_len=32,
        salt_len=16
    )
    
    hashed_passwords = []
    
    for i, line in enumerate(lines):
        line = line.strip()
        if not line or line.startswith('#'):
            continue
        
        if ':' not in line:
            print(f"ERROR: Line {i+1} invalid format (need username:password): {line}", file=sys.stderr)
            sys.exit(1)
        
        username, password = line.split(':', 1)
        
        try:
            # Hash the password
            hashed = hasher.hash(password)
            hashed_passwords.append(f"{username}:{hashed}")
            print(f"✓ Hashed password for user: {username}", file=sys.stderr)
        except Exception as e:
            print(f"ERROR: Failed to hash password for {username}: {e}", file=sys.stderr)
            sys.exit(1)
    
    # Create passwd file content
    passwd_content = '\n'.join(hashed_passwords) + '\n'
    
    # Base64 encode
    passwd_b64 = base64.b64encode(passwd_content.encode()).decode()
    
    print("=" * 70)
    print("MOSQUITTO HASHED PASSWORD OUTPUT")
    print("=" * 70)
    print("\nBase64-encoded hashed passwords (for Kubernetes Secret):")
    print("-" * 70)
    print(f"passwd: {passwd_b64}")
    print("-" * 70)
    
    print("\nUpdate your Secret resource:")
    print("""
apiVersion: v1
kind: Secret
metadata:
  name: mosquitto-users
  namespace: <your-namespace>
type: Opaque
data:
  passwd: <PASTE_BASE64_OUTPUT_FROM_SCRIPT>
  acl: <base64-encoded ACL file>
""")
    
    print("\nReference (hashed passwords - safe to view):")
    for line in hashed_passwords:
        print(f"  {line}")
    
    print("\n" + "=" * 70)
    return passwd_b64

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("USAGE: python3 generate-passwords.py <plaintext-password-file>")
        print("\nExample password file format:")
        print("  hassio:MyPassword123")
        print("  z2m:AnotherPassword456")
        print("  device:DevicePassword789")
        print("\nDependencies:")
        print("  pip3 install argon2-cffi")
        print("\nNOTE: Never commit plain-text password files to git!")
        sys.exit(1)
    
    plaintext_file = sys.argv[1]
    generate_from_file(plaintext_file)

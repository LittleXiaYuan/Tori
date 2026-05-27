from setuptools import setup

# Metadata lives in pyproject.toml. This shim keeps older tooling that still
# invokes `python setup.py ...` from failing before the package is fully migrated.
setup()

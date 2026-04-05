from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="conduit-client",
    version="0.1.0",
    author="Conduit Contributors",
    description="Python client library for Conduit Exchange workers",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/tonyd33/conduit",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: Apache Software License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
    python_requires=">=3.9",
    install_requires=[
        "nats-py>=2.8.0",
    ],
)

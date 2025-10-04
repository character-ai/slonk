from setuptools import setup, find_packages

with open("README.md") as f:
    readme = f.read()

with open("requirements.txt") as f:
    requirements = [line for line in f.read().splitlines() if not line.startswith("-")]


setup(
    name="slonk",
    version="0.0.1",
    description="Slonk tool",
    url="https://github.com/your-org/your-k8s-repo/tree/main/users/slonk",  # TODO: Replace with your repository URL
    long_description=readme,
    long_description_content_type="text/markdown",
    python_requires=">=3.8",
    install_requires=requirements,
    entry_points={
        "console_scripts": ["slonk=slonk.__main__:main"],
    },
    packages=find_packages(),
)

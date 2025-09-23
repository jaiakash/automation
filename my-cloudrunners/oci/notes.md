

## Oracle cloud set up

```
COMPARTMENT_ID=ocid1.compartment.oc1..aaaaaaaazcfftdqqpqguwkpnk5pq3qxnav6olpodrz33sqz55lumxu6nie3q

# List availability-domains:
oci iam availability-domain list
AVAILABILITY_DOMAIN=tdbQ:US-ASHBURN-AD-1

# Pick the correct subnet for the AD
oci network vcn list --compartment-id ${COMPARTMENT_ID}
oci network vcn create --compartment-id ${COMPARTMENT_ID} --cidr-blocks '["10.0.0.0/16"]'

VCN_ID=ocid1.vcn.oc1.iad.amaaaaaazljd6pqa5xledbeicpayh3mfaiz2txq63q4rdi7cyc5oo432oola

oci network subnet list --compartment-id ${COMPARTMENT_ID}
oci network subnet create --compartment-id ${COMPARTMENT_ID} --cidr-block 10.0.0.0/16 --vcn-id ${VCN_ID}

SUBNET_ID=ocid1.subnet.oc1.iad.aaaaaaaa2z3oherkse4dapgrzqesa2p7pocvyl2vj5zcd6wgwftdluwpm4ga
oci network subnet get --subnet-id ${SUBNET_ID}

ROUTE_TABLE_ID=ocid1.routetable.oc1.iad.aaaaaaaawesua3udj7baf47yzablrjdxyvzc34indfnmo5ltms2cc6zwc7dq

oci network route-table get --rt-id ${ROUTE_TABLE_ID}

oci network internet-gateway create --vcn-id ${VCN_ID} --compartment-id ${COMPARTMENT_ID} --is-enabled true

INTERNET_GATEWAY_ID=ocid1.internetgateway.oc1.iad.aaaaaaaaotcs2xmbni4val4dtn34w4jv3piuovudual2rkufh7b54y5gy2gq
ROUTE_RULES=$(cat <<EOF
[
  {
    "cidrBlock": "0.0.0.0/0",
    "networkEntityId": "$INTERNET_GATEWAY_ID"
  }
]
EOF
)
oci network route-table update --rt-id ${ROUTE_TABLE_ID}  --route-rules "${ROUTE_RULES}" --force

# Allow SSH
SECURITY_LIST_ID=$(oci network subnet get  --subnet-id ${SUBNET_ID} | jq -r '.data."security-list-ids"[0]')
oci network security-list update \
    --security-list-id ${SECURITY_LIST_ID} \
    --ingress-security-rules '[{"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 22, "max": 22}}}]'
```


## Create OCI disk image

```
IMAGE_TAG=$(date +%Y%m%d-%H%M%S)

BASE_IMAGE_AMD64=$(oci compute image list --compartment-id ${COMPARTMENT_ID} --display-name Canonical-Ubuntu-24.04-Minimal-2025.01.31-1 --query "data[0].id" --raw-output)
echo "Using BASE_IMAGE_AMD64 ${BASE_IMAGE_AMD64}"
go run ./tools/gha-imagebuilder-oci/  --base-image ${BASE_IMAGE_AMD64} --create-image-name gha-${IMAGE_TAG}-amd64 --availability-domain ${AVAILABILITY_DOMAIN} --subnet ${SUBNET_ID} --shape VM.Standard.E5.Flex
AMD64_DISK_ID=$(oci compute image list --display-name gha-${IMAGE_TAG}-amd64 --compartment-id ${COMPARTMENT_ID} --query "data[0].id" --raw-output)
echo "AMD64_DISK_ID is ${AMD64_DISK_ID}"

BASE_IMAGE_ARM64=$(oci compute image list --compartment-id ${COMPARTMENT_ID} --display-name Canonical-Ubuntu-24.04-Minimal-aarch64-2025.01.31-1 --query "data[0].id" --raw-output)
echo "Using BASE_IMAGE_ARM64 ${BASE_IMAGE_ARM64}"
go run ./tools/gha-imagebuilder-oci/  --base-image ${BASE_IMAGE_ARM64} --create-image-name gha-${IMAGE_TAG}-arm64 --availability-domain ${AVAILABILITY_DOMAIN} --subnet ${SUBNET_ID} --shape VM.Standard.A1.Flex
ARM64_DISK_ID=$(oci compute image list --display-name gha-${IMAGE_TAG}-amd64 --compartment-id ${COMPARTMENT_ID} --query "data[0].id" --raw-output)
echo "ARM64_DISK_ID is ${ARM64_DISK_ID}"
```

## Configuring github-actions-runner

# Upload runner image to gcr
# TODO: docker buildx build --push -t gcr.io/${PROJECT_ID}/gha-cloudrunners-gcp:latest .

# TODO: Instructions for hooking up to github-actions-runner

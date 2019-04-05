package account

import (
	"context"
	"strconv"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2017-09-01/skus"
	"github.com/Azure/azure-sdk-for-go/services/preview/subscription/mgmt/2018-03-01-preview/subscription"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/compute/v1"

	"github.com/supergiant/control/pkg/clouds"
	"github.com/supergiant/control/pkg/model"
	"github.com/supergiant/control/pkg/sgerrors"
	"github.com/supergiant/control/pkg/workflows/steps"
)

var fakeErr = errors.New("fake error")

type mockSizeService struct {
	mock.Mock
}

type mockRegionService struct {
	mock.Mock
}

func (m *mockSizeService) List(ctx context.Context, options *godo.ListOptions) ([]godo.Size,
	*godo.Response, error) {
	args := m.Called(ctx, options)
	val, ok := args.Get(0).([]godo.Size)
	if !ok {
		return nil, nil, args.Error(1)
	}
	return val, nil, args.Error(1)
}

func (m *mockRegionService) List(ctx context.Context, options *godo.ListOptions) ([]godo.Region,
	*godo.Response, error) {
	args := m.Called(ctx, options)
	val, ok := args.Get(0).([]godo.Region)
	if !ok {
		return nil, nil, args.Error(1)
	}
	return val, nil, args.Error(1)
}

type fakeSubscriptions struct {
	list subscription.LocationListResult
	err  error
}

func (c fakeSubscriptions) ListLocations(ctx context.Context, subscriptionID string) (subscription.LocationListResult, error) {
	return c.list, c.err
}

type fakeSKUSClient struct {
	res skus.ResourceSkusResultIterator
	err error
}

func (c fakeSKUSClient) ListComplete(ctx context.Context) (skus.ResourceSkusResultIterator, error) {
	return c.res, c.err
}

func TestGetRegionFinder(t *testing.T) {
	testCases := []struct {
		account *model.CloudAccount
		err     error
	}{
		{
			account: nil,
			err:     ErrNilAccount,
		},
		{
			account: &model.CloudAccount{
				Provider: "Unknown",
			},
			err: ErrUnsupportedProvider,
		},
		{
			account: &model.CloudAccount{
				Provider: clouds.DigitalOcean,
				Credentials: map[string]string{
					"dumb": "1234",
				},
			},
			err: sgerrors.ErrInvalidCredentials,
		},
		{
			account: &model.CloudAccount{
				Provider: clouds.DigitalOcean,
				Credentials: map[string]string{
					"accessToken": "1234",
				},
			},
		},
	}

	for _, testCase := range testCases {
		rf, err := NewRegionsGetter(testCase.account, &steps.Config{})

		if err != testCase.err {
			t.Errorf("expected error %v actual %v", testCase.err, err)
		}

		if err == nil && rf == nil {
			t.Error("region finder must not be nil")
		}
	}
}

func TestFind(t *testing.T) {
	errRegion := errors.New("region")
	errSize := errors.New("sizes")

	testCases := []struct {
		regions []godo.Region
		sizes   []godo.Size

		sizeErr   error
		regionErr error

		expectedErr    error
		expectedOutput *RegionSizes
	}{
		{
			sizeErr:     errSize,
			expectedErr: errSize,
		},
		{
			regionErr:   errRegion,
			expectedErr: errRegion,
		},
		{
			sizes:   []godo.Size{},
			regions: []godo.Region{},

			regionErr: nil,
			sizeErr:   nil,

			expectedErr: nil,
			expectedOutput: &RegionSizes{
				Regions: []*Region{},
				Sizes:   map[string]interface{}{},
			},
		},
	}

	for _, testCase := range testCases {
		sizeSvc := &mockSizeService{}
		sizeSvc.On("List", mock.Anything, mock.Anything).
			Return(testCase.sizes, testCase.sizeErr)

		regionSvc := &mockRegionService{}
		regionSvc.On("List", mock.Anything, mock.Anything).
			Return(testCase.regions, testCase.regionErr)

		rf := digitalOceanRegionFinder{
			getServices: func() (godo.SizesService, godo.RegionsService) {
				return sizeSvc, regionSvc
			},
		}

		regionSizes, err := rf.GetRegions(context.Background())

		if err != testCase.expectedErr {
			t.Errorf("expected error %v actual %v", testCase.expectedErr, err)
		}

		if err == nil && regionSizes == nil {
			t.Error("output must not be nil")
		}

		if testCase.expectedErr == nil {
			if regionSizes.Provider != clouds.DigitalOcean {
				t.Errorf("Wrong cloud provider expected %s actual %s",
					clouds.DigitalOcean, regionSizes.Provider)
			}

			if len(regionSizes.Regions) != len(testCase.regions) {
				t.Errorf("wrong count of regions expected %d actual %d",
					len(testCase.regions), len(regionSizes.Regions))
			}

			if len(regionSizes.Sizes) != len(testCase.sizes) {
				t.Errorf("wrong count of sizes expected %d actual %d",
					len(testCase.sizes), len(regionSizes.Sizes))
			}
		}
	}
}

func TestConvertSize(t *testing.T) {
	memory := 16
	vcpus := 4

	size := godo.Size{
		Slug:   "test",
		Memory: memory,
		Vcpus:  vcpus,
	}

	nodeSizes := map[string]interface{}{}
	convertSize(size, nodeSizes)

	if _, ok := nodeSizes[size.Slug]; !ok {
		t.Errorf("size with slug %s not found in %v",
			size.Slug, nodeSizes)
		return
	}

	s, ok := nodeSizes[size.Slug].(Size)

	if !ok {
		t.Errorf("Wrong type of value %v expected Size", nodeSizes[size.Slug])
		return
	}

	if s.CPU != strconv.Itoa(size.Vcpus) {
		t.Errorf("wrong vcpu count expected %d actual %s", size.Vcpus, s.CPU)
	}

	if s.RAM != strconv.Itoa(size.Memory) {
		t.Errorf("wrong memory count expected %d actual %s", size.Memory, s.RAM)
	}
}

func TestConvertRegions(t *testing.T) {
	region := godo.Region{
		Slug:  "fra1",
		Name:  "Frankfurt1",
		Sizes: []string{"size-1", "size-2"},
	}

	r := convertRegion(region)

	if r.Name != region.Name {
		t.Errorf("Wrong name of region expected %s actual %s", region.Name, r.Name)
	}

	if r.ID != region.Slug {
		t.Errorf("Wrong ID of region expected %s actual %s", region.Slug, r.ID)
	}

	if len(r.AvailableSizes) != len(region.Sizes) {
		t.Errorf("Wrong count of sizes expected %d actual %d",
			len(region.Sizes), len(r.AvailableSizes))
	}
}

func TestGCEResourceFinder_GetRegions(t *testing.T) {
	testCases := []struct {
		projectID  string
		err        error
		regionList *compute.RegionList
	}{
		{
			projectID:  "test",
			err:        sgerrors.ErrNotFound,
			regionList: nil,
		},
		{
			projectID: "test",
			err:       nil,
			regionList: &compute.RegionList{
				Items: []*compute.Region{
					{
						Name: "europe-1",
					},
					{
						Name: "ap-north-2",
					},
					{
						Name: "us-west-3",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		finder := &GCEResourceFinder{
			client: nil,
			config: steps.Config{
				GCEConfig: steps.GCEConfig{
					ServiceAccount: steps.ServiceAccount{
						ProjectID: testCase.projectID,
					},
				},
			},
			listRegions: func(client *compute.Service, projectID string) (*compute.RegionList, error) {
				if projectID != testCase.projectID {
					t.Errorf("Expected projectID %s actual %s",
						testCase.projectID, projectID)
				}

				return testCase.regionList, testCase.err
			},
		}

		regionSizes, err := finder.GetRegions(context.Background())

		if testCase.err != nil && !sgerrors.IsNotFound(err) {
			t.Errorf("Expected err %v actual %v", testCase.err, err)
		}

		if testCase.err == nil {
			if len(regionSizes.Regions) != len(testCase.regionList.Items) {
				t.Errorf("Wrong count of regions expected %d actual %d",
					len(testCase.regionList.Items), len(regionSizes.Regions))
			}
		}
	}
}

func TestGCEResourceFinder_GetZones(t *testing.T) {
	testCases := []struct {
		projectID string
		regionID  string
		err       error
		region    *compute.Region
	}{
		{
			projectID: "test",
			regionID:  "us-east1",
			err:       sgerrors.ErrNotFound,
			region:    nil,
		},
		{
			projectID: "test",
			regionID:  "us-east1",
			err:       nil,
			region: &compute.Region{
				Zones: []string{"us-east1-b", "us-east1-c", "us-east1-d"},
			},
		},
	}

	for _, testCase := range testCases {
		finder := &GCEResourceFinder{
			client: nil,
			config: steps.Config{
				GCEConfig: steps.GCEConfig{
					Region: testCase.regionID,
					ServiceAccount: steps.ServiceAccount{
						ProjectID: testCase.projectID,
					},
				},
			},
			getRegion: func(client *compute.Service, projectID, regionID string) (*compute.Region, error) {
				if projectID != testCase.projectID {
					t.Errorf("Expected projectID %s actual %s",
						testCase.projectID, projectID)
				}

				if regionID != testCase.regionID {
					t.Errorf("Expected regionID %s actual %s",
						testCase.regionID, regionID)
				}

				return testCase.region, testCase.err
			},
		}

		config := steps.Config{
			GCEConfig: steps.GCEConfig{
				ServiceAccount: steps.ServiceAccount{
					ProjectID: testCase.projectID,
				},
				Region:    testCase.regionID,
			},
		}
		zones, err := finder.GetZones(context.Background(), config)

		if testCase.err != nil && !sgerrors.IsNotFound(err) {
			t.Errorf("Expected err %v actual %v", testCase.err, err)
		}

		if testCase.err == nil {
			if len(zones) != len(testCase.region.Zones) {
				t.Errorf("Wrong count of zones expected %d actual %d",
					len(testCase.region.Zones), len(zones))
			}
		}
	}
}

func TestGCEResourceFinder_GetTypes(t *testing.T) {
	testCases := []struct {
		projectID string
		zoneID    string
		err       error
		types     *compute.MachineTypeList
	}{
		{
			projectID: "test",
			zoneID:    "us-east33-a",
			err:       sgerrors.ErrNotFound,
			types:     nil,
		},
		{
			projectID: "test",
			zoneID:    "us-east1-b",
			err:       nil,
			types: &compute.MachineTypeList{
				Items: []*compute.MachineType{
					{
						Name: "n1-standard-8",
					},
					{
						Name: "n1-highmem-32",
					},
					{
						Name: "n1-highcpu-96",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		config := steps.Config{
			GCEConfig: steps.GCEConfig{
				ServiceAccount: steps.ServiceAccount{
					ProjectID: testCase.projectID,
				},
				AvailabilityZone: testCase.zoneID,
			},
		}

		finder := &GCEResourceFinder{
			client: nil,
			config: config,
			listMachineTypes: func(client *compute.Service, projectID, zoneID string) (*compute.MachineTypeList, error) {
				if projectID != testCase.projectID {
					t.Errorf("Expected projectID %s actual %s",
						testCase.projectID, projectID)
				}

				if zoneID != testCase.zoneID {
					t.Errorf("Expected types %s actual %s",
						testCase.zoneID, zoneID)
				}

				return testCase.types, testCase.err
			},
		}

		types, err := finder.GetTypes(context.Background(), config)

		if testCase.err != nil && !sgerrors.IsNotFound(err) {
			t.Errorf("Expected err %v actual %v", testCase.err, err)
		}

		if testCase.err == nil {
			if len(types) != len(testCase.types.Items) {
				t.Errorf("Wrong count of types expected %d actual %d",
					len(testCase.types.Items), len(types))
			}
		}
	}
}

func TestAWSFinder_GetRegions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		finder AWSFinder
		expRes *RegionSizes
		expErr error
	}{
		{
			name: "get regions",
			finder: AWSFinder{
				machines: awsMachines,
			},
			expRes: &RegionSizes{
				Provider: clouds.AWS,
				Regions:  toRegions(awsMachines.Regions()),
			},
		},
	} {
		res, err := tc.finder.GetRegions(context.Background())
		require.Equalf(t, tc.expErr, errors.Cause(err), "TC: %s", tc.name)

		require.Equalf(t, tc.expRes, res, "TC: %s", tc.name)
	}
}

func TestAWSFinder_GetZones(t *testing.T) {
	testCases := []struct {
		err  error
		resp *ec2.DescribeAvailabilityZonesOutput
	}{
		{
			err:  sgerrors.ErrNotFound,
			resp: nil,
		},
		{
			err: nil,
			resp: &ec2.DescribeAvailabilityZonesOutput{
				AvailabilityZones: []*ec2.AvailabilityZone{
					{
						ZoneName: aws.String("ap-northeast1-b"),
					},
					{
						ZoneName: aws.String("eu-west2-a"),
					},
					{
						ZoneName: aws.String("us-west1-c"),
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		awsFinder := &AWSFinder{
			getZones: func(ctx context.Context, client *ec2.EC2,
				input *ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error) {
				return testCase.resp, testCase.err
			},
		}

		resp, err := awsFinder.GetZones(context.Background(), steps.Config{})

		if testCase.err != nil && !sgerrors.IsNotFound(err) {
			t.Errorf("wrong error expected %v actual %v", testCase.err, err)
		}

		if err == nil && len(resp) != len(testCase.resp.AvailabilityZones) {
			t.Errorf("Wrong count of regions expected %d actual %d",
				len(testCase.resp.AvailabilityZones), len(resp))
		}
	}
}

func AWSEUWEST1Types() []string {
	r, _ := awsMachines.RegionTypes("eu-west-1")
	return r
}

func TestAWSFinder_GetTypes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		finder AWSFinder
		in     steps.Config
		expRes []string
		expErr error
	}{
		{
			name: "unknown region",
			finder: AWSFinder{
				machines: awsMachines,
			},
			expErr: sgerrors.ErrRawError,
		},
		{
			name: "region: eu-west-1",
			finder: AWSFinder{
				machines: awsMachines,
			},
			in: steps.Config{
				AWSConfig: steps.AWSConfig{
					Region: "eu-west-1",
				},
			},
			expRes: AWSEUWEST1Types(),
		},
	} {
		res, err := tc.finder.GetTypes(context.Background(), tc.in)
		require.Equalf(t, tc.expErr, errors.Cause(err), "TC: %s", tc.name)

		require.Equalf(t, tc.expRes, res, "TC: %s", tc.name)
	}
}

func TestAzureFinder_GetRegions(t *testing.T) {
	// used for the success test mock
	var zero int

	for _, tc := range []struct {
		name        string
		f           AzureFinder
		expectedRes *RegionSizes
		expectedErr error
	}{
		{
			name: "list locations: error",
			f: AzureFinder{
				subscriptionsClient: fakeSubscriptions{
					err: fakeErr,
				},
			},
			expectedErr: fakeErr,
		},
		{
			name: "list locations: nil values",
			f: AzureFinder{
				subscriptionsClient: fakeSubscriptions{},
			},
			expectedErr: sgerrors.ErrNilEntity,
		},
		{
			name: "get vm sizes: ListComplete error",
			f: AzureFinder{
				subscriptionsClient: fakeSubscriptions{
					list: subscription.LocationListResult{
						Value: &([]subscription.Location{}),
					},
				},
				skusClient: fakeSKUSClient{
					err: fakeErr,
				},
			},
			expectedErr: fakeErr,
		},
		{
			name: "get vm sizes: NextWithContext error",
			f: AzureFinder{
				subscriptionsClient: fakeSubscriptions{
					list: subscription.LocationListResult{
						Value: &([]subscription.Location{
							{
								Name:        to.StringPtr("euwest1"),
								DisplayName: to.StringPtr("eu west 1"),
							},
						}),
					},
				},
				skusClient: fakeSKUSClient{
					res: skus.NewResourceSkusResultIterator(skus.NewResourceSkusResultPage(
						func(context.Context, skus.ResourceSkusResult) (skus.ResourceSkusResult, error) {
							return skus.ResourceSkusResult{}, fakeErr
						},
					)),
				},
			},
			expectedErr: fakeErr,
		},
		{
			name: "get vm sizes: success",
			f: AzureFinder{
				subscriptionsClient: fakeSubscriptions{
					list: subscription.LocationListResult{
						Value: &([]subscription.Location{
							{
								Name:        to.StringPtr("euwest1"),
								DisplayName: to.StringPtr("eu west 1"),
							},
						}),
					},
				},
				skusClient: fakeSKUSClient{
					res: skus.NewResourceSkusResultIterator(skus.NewResourceSkusResultPage(
						func(context.Context, skus.ResourceSkusResult) (skus.ResourceSkusResult, error) {
							if zero == 0 {
								zero++
								return skus.ResourceSkusResult{
									Value: &([]skus.ResourceSku{
										{
											Name:         to.StringPtr(""),
											Size:         to.StringPtr("size0"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size1name"),
											Size:         to.StringPtr("size1"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size2name"),
											Size:         to.StringPtr(""),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size3name"),
											Size:         to.StringPtr("size3"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size4name"),
											Size:         to.StringPtr("size4"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"useast1"}),
										},
									}),
								}, nil
							}
							return skus.ResourceSkusResult{}, nil
						},
					)),
				},
			},
			expectedRes: &RegionSizes{
				Provider: clouds.Azure,
				Regions: []*Region{
					{
						ID:             "euwest1",
						Name:           "eu west 1",
						AvailableSizes: []string{"size1name", "size3name"},
					},
				},
				Sizes: map[string]interface{}{
					"size1name": skus.ResourceSku{
						Name:         to.StringPtr("size1name"),
						Size:         to.StringPtr("size1"),
						ResourceType: to.StringPtr(VMResourceType),
						Locations:    to.StringSlicePtr([]string{"euwest1"}),
					},
					"size3name": skus.ResourceSku{
						Name:         to.StringPtr("size3name"),
						Size:         to.StringPtr("size3"),
						ResourceType: to.StringPtr(VMResourceType),
						Locations:    to.StringSlicePtr([]string{"euwest1"}),
					},
					"size4name": skus.ResourceSku{
						Name:         to.StringPtr("size4name"),
						Size:         to.StringPtr("size4"),
						ResourceType: to.StringPtr(VMResourceType),
						Locations:    to.StringSlicePtr([]string{"useast1"}),
					},
				},
			},
		},
	} {
		rs, err := tc.f.GetRegions(context.Background())

		require.Equalf(t, tc.expectedRes, rs, "TC: %s: check result", tc.name)
		require.Equalf(t, tc.expectedErr, errors.Cause(err), "TC: %s: check error", tc.name)
	}
}

func TestAzureFinder_GetTypes(t *testing.T) {
	// used for the success test mock
	var zero int

	for _, tc := range []struct {
		name        string
		f           AzureFinder
		expectedRes []string
		expectedErr error
	}{
		{
			name: "get vm sizes: ListComplete error",
			f: AzureFinder{
				skusClient: fakeSKUSClient{
					err: fakeErr,
				},
			},
			expectedErr: fakeErr,
		},
		{
			name: "get vm sizes: NextWithContext error",
			f: AzureFinder{
				skusClient: fakeSKUSClient{
					res: skus.NewResourceSkusResultIterator(skus.NewResourceSkusResultPage(
						func(context.Context, skus.ResourceSkusResult) (skus.ResourceSkusResult, error) {
							return skus.ResourceSkusResult{}, fakeErr
						},
					)),
				},
			},
			expectedErr: fakeErr,
		},
		{
			name: "get vm sizes: success",
			f: AzureFinder{
				skusClient: fakeSKUSClient{
					res: skus.NewResourceSkusResultIterator(skus.NewResourceSkusResultPage(
						func(context.Context, skus.ResourceSkusResult) (skus.ResourceSkusResult, error) {
							if zero == 0 {
								zero++
								return skus.ResourceSkusResult{
									Value: &([]skus.ResourceSku{
										{
											Name:         to.StringPtr(""),
											Size:         to.StringPtr("size0"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size1name"),
											Size:         to.StringPtr("size1"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size2name"),
											Size:         to.StringPtr(""),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size3name"),
											Size:         to.StringPtr("size3"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"euwest1"}),
										},
										{
											Name:         to.StringPtr("size4name"),
											Size:         to.StringPtr("size4"),
											ResourceType: to.StringPtr(VMResourceType),
											Locations:    to.StringSlicePtr([]string{"useast1"}),
										},
									}),
								}, nil
							}
							return skus.ResourceSkusResult{}, nil
						},
					)),
				},
				location: "euwest1",
			},
			expectedRes: []string{"size1name", "size3name"},
		},
	} {
		rs, err := tc.f.GetTypes(context.Background(), steps.Config{})

		require.Equalf(t, tc.expectedRes, rs, "TC: %s: check result", tc.name)
		require.Equalf(t, tc.expectedErr, errors.Cause(err), "TC: %s: check error", tc.name)
	}
}
